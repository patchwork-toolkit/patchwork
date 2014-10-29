package catalog

import (
	"net/http"
	"path/filepath"
	"strings"
)

// Serves static and all /static/ctx files as ld+json
func NewStaticHandler(staticDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.RequestURI, StaticLocation+"/ctx/") {
			w.Header().Set("Content-Type", "application/ld+json")
		}
		urlParts := strings.Split(req.URL.Path, "/")
		http.ServeFile(w, req, filepath.Join(staticDir, strings.Join(urlParts[2:], "/")))
	}
}
