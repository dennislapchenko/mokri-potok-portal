package httpapi

import (
	"io"
	"net/http"
)

// Photos live in the database, whatever they picture: a tool in the shed or a
// project half done. The browser shrinks them to ~1000 px first; the backend
// takes the raw bytes with an image Content-Type, caps them at 2 MB and serves
// them back only to a token, because <img> cannot carry a bearer header and the
// frontend fetches them into an object URL instead.

// readPhoto reads an image body or writes the refusal and returns ok=false.
func readPhoto(w http.ResponseWriter, r *http.Request) (body []byte, ct string, ok bool) {
	ct = r.Header.Get("Content-Type")
	if ct != "image/jpeg" && ct != "image/png" && ct != "image/webp" {
		writeErr(w, 415, "jpeg, png or webp")
		return nil, "", false
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
	if err != nil {
		writeErr(w, 413, "photo over 2 MB — the app should have shrunk it")
		return nil, "", false
	}
	return body, ct, true
}

// servePhoto writes one row's photo and photo_type columns.
func servePhoto(w http.ResponseWriter, row map[string]any) {
	var b []byte
	switch v := row["photo"].(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	}
	w.Header().Set("Content-Type", row["photo_type"].(string))
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Write(b)
}

// ---- project photos --------------------------------------------------------

// addProjectPhoto: any house may put a picture on any project — a project is
// the village's, the way its events are. The picture remembers who added it.
func (s *Server) addProjectPhoto(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	pid, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "bad id")
		return
	}
	p, err := s.st.One(r.Context(), `SELECT id FROM projects WHERE id=?`, pid)
	if err != nil {
		fail(w, err)
		return
	}
	if p == nil {
		writeErr(w, 404, "no such project")
		return
	}
	body, ct, ok := readPhoto(w, r)
	if !ok {
		return
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO project_photos(project_id, house_id, photo, photo_type) VALUES (?,?,?,?)`, pid, h.ID, body, ct)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id})
}

func (s *Server) getProjectPhoto(w http.ResponseWriter, r *http.Request) {
	id, _ := pathID(r)
	row, err := s.st.One(r.Context(), `SELECT photo, photo_type FROM project_photos WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if row == nil {
		writeErr(w, 404, "no photo")
		return
	}
	servePhoto(w, row)
}

// deleteProjectPhoto: the house that added it, the project's house, or a steward.
func (s *Server) deleteProjectPhoto(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, _ := pathID(r)
	row, err := s.st.One(r.Context(), `SELECT f.house_id, p.house_id AS project_house FROM project_photos f JOIN projects p ON p.id=f.project_id WHERE f.id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if row == nil {
		writeErr(w, 404, "no photo")
		return
	}
	if row["house_id"].(int64) != h.ID && row["project_house"].(int64) != h.ID && !h.IsSteward {
		writeErr(w, 403, "not yours")
		return
	}
	s.st.Exec(r.Context(), `DELETE FROM project_photos WHERE id=?`, id)
	w.WriteHeader(204)
}
