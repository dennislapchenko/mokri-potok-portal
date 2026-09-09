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

const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' blob:; font-src 'self'; connect-src 'self'; worker-src 'self'; manifest-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

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
		if p == "index.html" {
			// The page's own line on who it may talk to: itself. Inline styles
			// stay because React paints house colours into style attributes;
			// blob: because tool photos and the export arrive as blobs.
			// Nothing reaches a third host — the fonts are served from here,
			// the weather comes through /api. A CSP here rather than at the
			// edge ships with the frontend it describes and is tested with it.
			w.Header().Set("Content-Security-Policy", csp)
		}
		if strings.HasSuffix(p, ".webmanifest") {
			// Go's MIME table has no .webmanifest and distroless carries no
			// /etc/mime.types, so the manifest would go out sniffed as text/plain.
			w.Header().Set("Content-Type", "application/manifest+json")
		}
		switch {
		case strings.HasPrefix(p, "assets/"):
			// Vite puts a content hash in every asset name.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		case p == "index.html" || p == "sw.js" || p == "changelog.json":
			// The shell and the service worker decide what everything else is;
			// the changelog is written at build time and read after a deploy.
			w.Header().Set("Cache-Control", "no-cache")
		default:
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		files.ServeHTTP(w, r)
	})
}
