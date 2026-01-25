package sitedata

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/pkg/errors"
)

//go:embed templates/*
var Assets embed.FS

var debug bool
var debugRootDir string

func SetConfig(d bool, drd string) {
	debug = d
	debugRootDir = drd
}

func GetStaticHandler(prefix string) httprouter.Handle {
	prefix = strings.TrimPrefix(prefix, "/")

	return func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		if debug {
			fs := http.Dir(filepath.Join(debugRootDir, prefix))
			handler := http.StripPrefix("/"+prefix+"/", http.FileServer(fs))
			handler.ServeHTTP(w, r)
		} else {
			// Serve embedded files
			subFS, err := fs.Sub(Assets, "templates")
			if err != nil {
				http.Error(w, "Failed to load embedded files", http.StatusInternalServerError)
				return
			}
			handler := http.StripPrefix("/"+prefix+"/", http.FileServer(http.FS(subFS)))
			handler.ServeHTTP(w, r)
		}
	}
}

func AddRoutes(r *httprouter.Router, prefix string) {
	r.GET(prefix+"/*filepath", GetStaticHandler(prefix))
}

func LoadFilesInDir(dir string) (map[string]string, error) {
	dirData := map[string]string{}
	if debug {
		fullDir := filepath.Join(debugRootDir, dir)
		files, err := os.ReadDir(fullDir)
		if err != nil {
			return dirData, errors.Wrapf(err, "Error reading dir %v", debugRootDir)
		}
		for _, file := range files {
			data, err := os.ReadFile(filepath.Join(fullDir, file.Name()))
			if err != nil {
				return dirData, errors.Wrapf(err, "Error loading %v", file.Name())
			}
			dirData[file.Name()] = string(data)
		}
	} else {
		// Load from embedded files
		entries, err := fs.ReadDir(Assets, "templates")
		if err != nil {
			return dirData, errors.Wrapf(err, "Could not read embedded templates dir")
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				data, err := fs.ReadFile(Assets, filepath.Join("templates", entry.Name()))
				if err != nil {
					return dirData, errors.Wrapf(err, "Error loading embedded file %v", entry.Name())
				}
				dirData[entry.Name()] = string(data)
			}
		}
	}
	return dirData, nil
}
