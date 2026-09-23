package httpapi

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// A village may replace the pictures the page wears — today the backdrop
// behind the gate, frontend/public/backdrop.jpg — with its own, entered once
// from a file on the VM like the map:
//
//	docker exec -i <container> /server site-import backdrop < picture.jpg
//
// The row lives in site_files; with no row, the picture built into the image
// serves, so every village starts with the same one. The backdrop is public
// (it is behind the gate, before any login), so it is served with no token,
// as the built-in file is.

var siteNames = map[string]bool{"backdrop": true}

const siteCap = 2 << 20

// ImportSite reads one picture from `in` and upserts its row. The type is
// read off the bytes, not the filename, and only an image is taken.
func (s *Server) ImportSite(name string, in io.Reader) error {
	if !siteNames[name] {
		return fmt.Errorf("name must be backdrop, got %q", name)
	}
	b, err := io.ReadAll(io.LimitReader(in, siteCap+1))
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	if len(b) > siteCap {
		return fmt.Errorf("%s is over %d MB", name, siteCap>>20)
	}
	ct := http.DetectContentType(b)
	if ct != "image/jpeg" && ct != "image/png" && ct != "image/webp" {
		return fmt.Errorf("%s must be a jpeg, png or webp, this is %s", name, ct)
	}
	_, err = s.st.Exec(context.Background(), `INSERT INTO site_files (name, bytes, content_type, updated_at) VALUES (?,?,?,datetime('now'))
		ON CONFLICT(name) DO UPDATE SET bytes=excluded.bytes, content_type=excluded.content_type, updated_at=excluded.updated_at`, name, b, ct)
	return err
}

// serveSiteFile writes the village's own picture if it has one and reports
// whether it did; the caller falls back to the built-in file otherwise.
func (s *Server) serveSiteFile(w http.ResponseWriter, r *http.Request, name string) bool {
	meta, err := s.st.One(r.Context(), `SELECT content_type, updated_at FROM site_files WHERE name=?`, name)
	if err != nil || meta == nil {
		return false
	}
	etag := fmt.Sprintf(`"%s-%s"`, name, strings.ReplaceAll(fmt.Sprint(meta["updated_at"]), " ", "T"))
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, no-cache")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(304)
		return true
	}
	row, err := s.st.One(r.Context(), `SELECT bytes FROM site_files WHERE name=?`, name)
	if err != nil || row == nil {
		return false
	}
	w.Header().Set("Content-Type", meta["content_type"].(string))
	switch v := row["bytes"].(type) {
	case []byte:
		w.Write(v)
	case string:
		w.Write([]byte(v))
	}
	return true
}
