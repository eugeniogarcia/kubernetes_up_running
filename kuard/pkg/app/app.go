package app

import (
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"kuard/pkg/debugprobe"
	"kuard/pkg/dnsapi"
	"kuard/pkg/env"
	"kuard/pkg/htmlutils"
	"kuard/pkg/keygen"
	"kuard/pkg/memory"
	memqserver "kuard/pkg/memq/server"
	"kuard/pkg/sitedata"
	"kuard/pkg/version"

	"github.com/felixge/httpsnoop"
	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func init() {
	prometheus.MustRegister(requestDuration)
}

var requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "request_duration_seconds",
	Help:    "Time serving HTTP request",
	Buckets: prometheus.DefBuckets,
}, []string{"method", "route", "status_code"})

func promMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m := httpsnoop.CaptureMetrics(h, w, r)
		requestDuration.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(m.Code)).Observe(m.Duration.Seconds())
	})
}

func loggingMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

type pageContext struct {
	URLBase      string       `json:"urlBase"`
	Hostname     string       `json:"hostname"`
	Addrs        []string     `json:"addrs"`
	Version      string       `json:"version"`
	VersionColor template.CSS `json:"versionColor"`
	RequestDump  string       `json:"requestDump"`
	RequestProto string       `json:"requestProto"`
	RequestAddr  string       `json:"requestAddr"`
}

type App struct {
	c  Config
	tg *htmlutils.TemplateGroup

	m     *memory.MemoryAPI
	live  *debugprobe.Probe
	ready *debugprobe.Probe
	env   *env.Env
	dns   *dnsapi.DNSAPI
	kg    *keygen.KeyGen
	mq    *memqserver.Server

	r *httprouter.Router
}

func (k *App) getPageContext(r *http.Request, urlBase string) *pageContext {
	c := &pageContext{}
	c.URLBase = urlBase
	c.Hostname, _ = os.Hostname()

	addrs, _ := net.InterfaceAddrs()
	c.Addrs = []string{}
	for _, addr := range addrs {
		// check the address type and if it is not a loopback
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				c.Addrs = append(c.Addrs, ipnet.IP.String())
			}
		}
	}

	c.Version = version.VERSION
	c.VersionColor = template.CSS(htmlutils.ColorFromString(version.VERSION))
	reqDump, _ := httputil.DumpRequest(r, false)
	c.RequestDump = strings.TrimSpace(string(reqDump))
	c.RequestProto = r.Proto
	c.RequestAddr = r.RemoteAddr

	return c
}

func (k *App) getRootHandler(urlBase string) httprouter.Handle {
	return httprouter.Handle(func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		k.tg.Render(w, "index.html", k.getPageContext(r, urlBase))
	})
}

// Exists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func (k *App) Run() {
	r := promMiddleware(loggingMiddleware(k.r))

	// Look to see if we can find TLS certs
	certFile := filepath.Join(k.c.TLSDir, "kuard.crt")
	keyFile := filepath.Join(k.c.TLSDir, "kuard.key")
	if fileExists(certFile) && fileExists(keyFile) {
		go func() {
			log.Printf("Serving HTTPS on %v", k.c.TLSAddr)
			log.Fatal(http.ListenAndServeTLS(k.c.TLSAddr, certFile, keyFile, r))
		}()
	} else {
		log.Printf("Could not find certificates to serve TLS")
	}

	log.Printf("Serving on HTTP on %v", k.c.ServeAddr)
	log.Fatal(http.ListenAndServe(k.c.ServeAddr, r))
}

func NewApp() *App {
	k := &App{
		tg: &htmlutils.TemplateGroup{},
		r:  httprouter.New(),
	}

	// Init all of the subcomponents

	router := k.r
	k.m = memory.New()
	k.live = debugprobe.New()
	k.ready = debugprobe.New()
	k.env = env.New()
	k.dns = dnsapi.New()
	k.kg = keygen.New()
	k.mq = memqserver.NewServer()

	// Add handlers
	for _, prefix := range []string{"", "/a", "/b", "/c"} {
		// capture loop variable to avoid accidental reuse in closures
		pfx := prefix
		rootHandler := k.getRootHandler(pfx)
		router.GET(pfx+"/", rootHandler)
		router.GET(pfx+"/-/*path", rootHandler)

		router.Handler("GET", pfx+"/metrics", promhttp.Handler())

		// Add the static files
		sitedata.AddRoutes(router, pfx+"/built")
		sitedata.AddRoutes(router, pfx+"/static")

		// Custom file system browser handler that enlarges text for directory listings.
		router.Handler("GET", pfx+"/fs/*filepath",
			http.StripPrefix(pfx+"/fs", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Serve files from the container root. r.URL.Path is the path after the StripPrefix.
				// Use an absolute path so the FS browser can access the full filesystem.
				p := filepath.Clean("/" + r.URL.Path)

				info, err := os.Stat(p)
				if err != nil {
					http.NotFound(w, r)
					return
				}
				if !info.IsDir() {
					http.ServeFile(w, r, p)
					return
				}

				entries, err := os.ReadDir(p)
				if err != nil {
					http.Error(w, "Failed to read directory", http.StatusInternalServerError)
					return
				}

				// base path that should appear in links (preserve external mount prefix)
				basePath := path.Join(pfx, "fs")
				if !strings.HasPrefix(basePath, "/") {
					basePath = "/" + basePath
				}

				// Normalize the requested (stripped) path so we don't accidentally re-insert
				// the /fs prefix if it somehow appears in r.URL.Path (previous bad links).
				rel := strings.TrimPrefix(r.URL.Path, "/")
				// If rel begins with the basePath (without leading slash), strip it.
				trimmedBase := strings.TrimPrefix(basePath, "/")
				if strings.HasPrefix(rel, trimmedBase) {
					rel = strings.TrimPrefix(rel, trimmedBase)
					rel = strings.TrimPrefix(rel, "/")
				}

				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				fmt.Fprintf(w, "<html><head><meta charset=\"utf-8\"><style>body{font-size:18px}</style></head><body><h1>Index of %s</h1><ul>", path.Join(basePath, rel))
				for _, e := range entries {
					name := e.Name()
					display := name
					if e.IsDir() {
						display = name + "/"
					}
					// Build href from normalized components so we never duplicate /fs
					if rel == "" {
						href := path.Join(basePath, name)
						if e.IsDir() {
							href = href + "/"
						}
						fmt.Fprintf(w, `<li><a href="%s">%s</a></li>`, href, display)
					} else {
						href := path.Join(basePath, rel, name)
						if e.IsDir() {
							href = href + "/"
						}
						fmt.Fprintf(w, `<li><a href="%s">%s</a></li>`, href, display)
					}
				}
				fmt.Fprint(w, "</ul></body></html>")
			})),
		)

		k.m.AddRoutes(router, prefix+"/mem")
		k.live.AddRoutes(router, prefix+"/healthy")
		k.ready.AddRoutes(router, prefix+"/ready")
		k.env.AddRoutes(router, prefix+"/env")
		k.dns.AddRoutes(router, prefix+"/dns")
		k.kg.AddRoutes(router, prefix+"/keygen")
		k.mq.AddRoutes(router, prefix+"/memq/server")
	}

	return k
}
