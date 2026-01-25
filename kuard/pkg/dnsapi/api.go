package dnsapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"kuard/pkg/apiutils"

	"github.com/julienschmidt/httprouter"
	"github.com/miekg/dns"
)

type DNSAPI struct {
}

// DNSResponse is returned from a GET to this API endpoing
type DNSResponse struct {
	Results string `json:"result"`
}

type DNSRequest struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func New() *DNSAPI {
	return &DNSAPI{}
}

func (e *DNSAPI) AddRoutes(r *httprouter.Router, base string) {
	r.POST(base+"/api", e.APIGet)
}

func (e *DNSAPI) APIGet(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	dreq := &DNSRequest{}

	err := json.NewDecoder(r.Body).Decode(dreq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := dnsQuery(dreq.Type, dreq.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dresp := &DNSResponse{
		Results: result,
	}

	apiutils.ServeJSON(w, dresp)
}

func dnsQuery(t string, name string) (string, error) {
	config, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		return "", err
	}

	c := new(dns.Client)
	m := new(dns.Msg)

	qtype, ok := dns.StringToType[strings.ToUpper(t)]
	if !ok {
		return "", fmt.Errorf("Unknown DNS type: %v", t)
	}

	if len(name) == 0 {
		name = "."
	}

	names := []string{}
	if dns.IsFqdn(name) {
		names = append(names, name)
	} else {
		// TODO: respect NDOTS
		for _, s := range config.Search {
			names = append(names, name+"."+s)
		}
		names = append(names, name)
	}

	var r *dns.Msg
	for _, name := range names {
		m.SetQuestion(dns.Fqdn(name), qtype)
		m.RecursionDesired = true
		r, _, err = c.Exchange(m, config.Servers[0]+":"+config.Port)
		if err != nil {
			return "", err
		}
		if len(r.Answer) > 0 {
			return r.String(), nil
		}
	}
	return r.String(), nil
}
