package api

import (
	"io/fs"
	"net/http"
	"strings"
)

// DistFS is set by main.go with the embedded web/dist files.
var DistFS fs.FS

// spaHandler serves the embedded SPA. For any route not matching a static file,
// it serves index.html (SPA client-side routing).
type spaHandler struct {
	fs     http.Handler
	static fs.FS
}

func newSPAHandler() http.Handler {
	if DistFS == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "SPA not built — run 'npm run build' in web/", 404)
		})
	}

	return &spaHandler{
		fs:     http.FileServer(http.FS(DistFS)),
		static: DistFS,
	}
}

func (h *spaHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Try to serve the actual file
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}

	// Check if file exists in embedded FS
	if _, err := fs.Stat(h.static, path); err == nil {
		h.fs.ServeHTTP(w, r)
		return
	}

	// For any other path, serve index.html (SPA routing)
	r.URL.Path = "/"
	h.fs.ServeHTTP(w, r)
}
