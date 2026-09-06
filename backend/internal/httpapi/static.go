package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

// The portal serves its own frontend. The Docker build drops Vite's output into
// this directory before `go build`, so one container answers both the API and
// the page — same origin, no CORS, no third host to keep alive. A binary built
// without that step carries the placeholder index.html committed here.

//go:embed all:web
var webFS embed.FS

func (s *Server) staticHandler() http.Handler {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(sub, p); err != nil {
			// Unknown path: hand back the shell. Routing is client-side.
			r = r.Clone(r.Context())
			r.URL.Path = "/"
			p = "index.html"
		}
		switch {
		case strings.HasPrefix(p, "assets/"):
			// Vite puts a content hash in every asset name.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case p == "index.html" || p == "sw.js":
			// The shell and the service worker decide what everything else is.
			w.Header().Set("Cache-Control", "no-cache")
		default:
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		files.ServeHTTP(w, r)
	})
}
