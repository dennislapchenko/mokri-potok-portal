package httpapi

import (
	"net/http"
	"strings"
)

// Tool photos and the wishlist. How a photo is read and served: photos.go.

var toolCategories = []string{"power", "garden", "other"}

func (s *Server) putToolPhoto(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "bad id")
		return
	}
	if !s.ownerOrSteward(w, r, "tools", id) {
		return
	}
	body, ct, ok := readPhoto(w, r)
	if !ok {
		return
	}
	if _, err := s.st.Exec(r.Context(), `UPDATE tools SET photo=?, photo_type=? WHERE id=?`, body, ct, id); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) deleteToolPhoto(w http.ResponseWriter, r *http.Request) {
	id, _ := pathID(r)
	if !s.ownerOrSteward(w, r, "tools", id) {
		return
	}
	s.st.Exec(r.Context(), `UPDATE tools SET photo=NULL, photo_type=NULL WHERE id=?`, id)
	w.WriteHeader(204)
}

func (s *Server) getToolPhoto(w http.ResponseWriter, r *http.Request) {
	id, _ := pathID(r)
	row, err := s.st.One(r.Context(), `SELECT photo, photo_type FROM tools WHERE id=? AND photo IS NOT NULL`, id)
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

// ---- wishlist ------------------------------------------------------------

func (s *Server) listWishes(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT x.*,`+houseJoin+`,
		(SELECT json_group_array(json_object('house_id', h2.id, 'name', h2.name, 'crest', h2.crest)) FROM wish_wants ww JOIN houses h2 ON h2.id=ww.house_id WHERE ww.wish_id=x.id) AS wants,
		(SELECT count(*) FROM wish_wants ww WHERE ww.wish_id=x.id AND ww.house_id=?) AS mine,
		(SELECT json_group_array(json_object('id', o.id, 'text', o.text, 'url', o.url, 'house_id', o.house_id, 'crest', oh.crest, 'color', oh.color, 'name', oh.name))
		 FROM wish_options o JOIN houses oh ON oh.id=o.house_id WHERE o.wish_id=x.id) AS options,
		(SELECT count(*) FROM comments c WHERE c.subject='wish' AND c.subject_id=x.id) AS comments
		FROM wishes x JOIN houses h ON h.id=x.house_id ORDER BY x.created_at DESC`, houseFrom(r).ID)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) createWish(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	m, err := readJSON(r)
	if err != nil || str(m, "text") == "" {
		writeErr(w, 400, "text required")
		return
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO wishes(house_id, text) VALUES (?,?)`, h.ID, str(m, "text"))
	if err != nil {
		fail(w, err)
		return
	}
	// The wisher wants it too, by definition.
	s.st.Exec(r.Context(), `INSERT OR IGNORE INTO wish_wants(wish_id, house_id) VALUES (?,?)`, id, h.ID)
	writeJSON(w, 201, map[string]any{"id": id})
}

// updateWish: any house adds or removes its own name under a wish.
func (s *Server) updateWish(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, _ := pathID(r)
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	if want, ok := m["want"].(bool); ok {
		if want {
			s.st.Exec(r.Context(), `INSERT OR IGNORE INTO wish_wants(wish_id, house_id) VALUES (?,?)`, id, h.ID)
		} else {
			s.st.Exec(r.Context(), `DELETE FROM wish_wants WHERE wish_id=? AND house_id=?`, id, h.ID)
		}
	}
	w.WriteHeader(204)
}

func validCategory(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	if contains(toolCategories, c) {
		return c
	}
	return "other"
}
