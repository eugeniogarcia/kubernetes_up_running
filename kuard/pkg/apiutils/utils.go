package apiutils

import (
	"encoding/json"
	"net/http"
	"time"
)

func ServeJSON(w http.ResponseWriter, o interface{}) {
	w.Header().Set("Content-Type", "application/json")
	NoCache(w)
	json.NewEncoder(w).Encode(o)
}

var epoch = time.Unix(0, 0).Format(time.RFC1123)
var noCacheHeaders = map[string]string{
	"Expires":       epoch,
	"Cache-Control": "no-cache, private, max-age=0",
	"Pragma":        "no-cache",
}

func NoCache(w http.ResponseWriter) {
	for k, v := range noCacheHeaders {
		w.Header().Set(k, v)
	}
}
