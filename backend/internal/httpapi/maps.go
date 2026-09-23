package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// The map's data — the cadastre and the modelled water — is two rows in
// map_files, entered once from a file on the VM with `/server map-import`, the
// way the codex is. It is served only to a token, like a photo: the cadastre
// is public data, but nothing on this origin is public. The bytes are
// validated on the way in, because an upload that is not a FeatureCollection
// would open every phone on an empty map until the next import.

const mapCap = 2 << 20 // both files together are under 400 KB; a cadastre ten times this size is a different product

var mapNames = map[string]string{"parcels": "application/geo+json", "water": "application/json"}

// listMap answers the caption and the empty state: which rows exist, with
// their source and snapshot, never the bytes.
func (s *Server) listMap(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT name, source, snapshot, updated_at FROM map_files ORDER BY name`)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

// getMap serves one row's bytes. The ETag is the row's updated_at, read
// first, so a phone that already holds the map gets a 304 and no 340 KB.
func (s *Server) getMap(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	ct, ok := mapNames[name]
	if !ok {
		writeErr(w, 404, "no such map file")
		return
	}
	meta, err := s.st.One(r.Context(), `SELECT updated_at FROM map_files WHERE name=?`, name)
	if err != nil {
		fail(w, err)
		return
	}
	if meta == nil {
		writeErr(w, 404, "this village has no "+name+" yet")
		return
	}
	etag := fmt.Sprintf(`"%s-%s"`, name, meta["updated_at"])
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(304)
		return
	}
	row, err := s.st.One(r.Context(), `SELECT bytes FROM map_files WHERE name=?`, name)
	if err != nil {
		fail(w, err)
		return
	}
	w.Header().Set("Content-Type", ct)
	switch v := row["bytes"].(type) {
	case []byte:
		w.Write(v)
	case string:
		w.Write([]byte(v))
	}
}

// ImportMap reads one map file from `in` and upserts its row. parcels must be
// a GeoJSON FeatureCollection with at least one feature and needs a source and
// a snapshot date for the caption; water must carry a `lines` array. Run on
// the VM: `docker exec -i <container> /server map-import parcels GURS 2026-08-15 < parcels.geojson`.
func (s *Server) ImportMap(name, source, snapshot string, in io.Reader) error {
	if _, ok := mapNames[name]; !ok {
		return fmt.Errorf("name must be parcels or water, got %q", name)
	}
	b, err := io.ReadAll(io.LimitReader(in, mapCap+1))
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	if len(b) > mapCap {
		return fmt.Errorf("%s is over %d MB", name, mapCap>>20)
	}
	switch name {
	case "parcels":
		var fc struct {
			Type     string            `json:"type"`
			Features []json.RawMessage `json:"features"`
		}
		if err := json.Unmarshal(b, &fc); err != nil || fc.Type != "FeatureCollection" || len(fc.Features) == 0 {
			return fmt.Errorf("parcels must be a GeoJSON FeatureCollection with features")
		}
		if source == "" || !isoDate.MatchString(snapshot) {
			return fmt.Errorf("parcels needs a source and a snapshot date: map-import parcels <source> <YYYY-MM-DD>")
		}
	case "water":
		var doc struct {
			Lines []json.RawMessage `json:"lines"`
		}
		if err := json.Unmarshal(b, &doc); err != nil || len(doc.Lines) == 0 {
			return fmt.Errorf("water must carry a lines array")
		}
		source, snapshot = "", ""
	}
	_, err = s.st.Exec(context.Background(), `INSERT INTO map_files (name, bytes, source, snapshot, updated_at) VALUES (?,?,?,?,datetime('now'))
		ON CONFLICT(name) DO UPDATE SET bytes=excluded.bytes, source=excluded.source, snapshot=excluded.snapshot, updated_at=excluded.updated_at`, name, b, source, snapshot)
	return err
}
