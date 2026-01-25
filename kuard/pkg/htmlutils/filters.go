package htmlutils

import (
	"encoding/json"
	"html/template"
	"time"

	"github.com/dustin/go-humanize"
)

func FuncMap() template.FuncMap {
	return template.FuncMap{
		"friendlytime": FriendlyTime,
		"reltime":      RelativeTime,
		"jsonstring":   JSONString,
	}
}

func FriendlyTime(t time.Time) string {
	return t.Format(time.Stamp)
}

func RelativeTime(t time.Time) string {
	return humanize.RelTime(t, time.Now(), "ago", "from now")
}

func JSONString(v interface{}) (template.JS, error) {
	a, err := json.Marshal(v)
	if err != nil {
		return template.JS(""), err
	}
	return template.JS(a), nil
}
