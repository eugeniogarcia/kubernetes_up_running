package keygen

import (
	"encoding/json"
	"net/http"

	"kuard/pkg/apiutils"

	"github.com/julienschmidt/httprouter"
)

// ProbeStatus is returned from a GET to this API endpoing
type KeyGenStatus struct {
	Config  Config    `json:"config"`
	History []History `json:"history"`
}

type History struct {
	ID   int    `json:"id"`
	Data string `json:"data"`
}

func (kg *KeyGen) APIPut(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
	c := Config{}

	err := json.NewDecoder(r.Body).Decode(&c)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	kg.LoadConfig(c)

	kg.APIGet(w, r, params)
}

func (kg *KeyGen) APIGet(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	kg.mu.Lock()
	defer kg.mu.Unlock()

	s := &KeyGenStatus{
		Config:  kg.config,
		History: kg.history,
	}

	apiutils.ServeJSON(w, s)
}
