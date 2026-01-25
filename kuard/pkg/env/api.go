package env

import (
	"net/http"
	"os"
	"strings"

	"kuard/pkg/apiutils"

	"github.com/julienschmidt/httprouter"
)

// EnvStatus is returned from a GET to this API endpoing
type EnvStatus struct {
	CommandLine []string          `json:"commandLine"`
	Env         map[string]string `json:"env"`
}

type Env struct {
}

func New() *Env {
	return &Env{}
}

func (e *Env) AddRoutes(r *httprouter.Router, base string) {
	r.GET(base+"/api", e.APIGet)
}

func (e *Env) APIGet(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	s := EnvStatus{}

	s.CommandLine = os.Args

	s.Env = map[string]string{}
	for _, e := range os.Environ() {
		splits := strings.SplitN(e, "=", 2)
		k, v := splits[0], splits[1]
		s.Env[k] = v
	}

	apiutils.ServeJSON(w, s)
}
