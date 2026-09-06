package httpapi

import (
	"net/http"
	"strconv"
	"strings"
)

// Comment threads, one implementation for every room that wants them: an event
// in the calendar, a wish in the tool shed. One reply level, like the board.
//
// Who hears about a new comment depends on the subject, and only on that — the
// push kind stays the room's own, so nobody's opt-out changes meaning.

var commentSubjects = []string{"event", "wish"}

func subjectOf(r *http.Request) (string, int64, bool) {
	s := r.PathValue("subject")
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || !contains(commentSubjects, s) {
		return "", 0, false
	}
	return s, id, true
}

func (s *Server) listThread(w http.ResponseWriter, r *http.Request) {
	subject, id, ok := subjectOf(r)
	if !ok {
		writeErr(w, 400, "bad subject")
		return
	}
	rows, err := s.st.Rows(r.Context(), `SELECT c.*,`+houseJoin+` FROM comments c JOIN houses h ON h.id=c.house_id
		WHERE c.subject=? AND c.subject_id=? ORDER BY c.created_at LIMIT 500`, subject, id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) createThreadComment(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	subject, id, ok := subjectOf(r)
	if !ok {
		writeErr(w, 400, "bad subject")
		return
	}
	m, err := readJSON(r)
	if err != nil || str(m, "body") == "" {
		writeErr(w, 400, "body required")
		return
	}
	table := map[string]string{"event": "events", "wish": "wishes"}[subject]
	owner, err := s.st.One(r.Context(), `SELECT house_id, `+map[string]string{"event": "title", "wish": "text"}[subject]+` AS label FROM `+table+` WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if owner == nil {
		writeErr(w, 404, "no such "+subject)
		return
	}
	// A reply belongs to the same thread, and one level deep: a reply to a
	// reply hangs off the root.
	var parent any
	if v, ok := m["parent_id"].(float64); ok && v > 0 {
		p, _ := s.st.One(r.Context(), `SELECT subject, subject_id, parent_id FROM comments WHERE id=?`, int64(v))
		if p == nil || p["subject"] != subject || p["subject_id"].(int64) != id {
			writeErr(w, 400, "no such comment here")
			return
		}
		if p["parent_id"] != nil {
			parent = p["parent_id"]
		} else {
			parent = int64(v)
		}
	}
	cid, err := s.st.Exec(r.Context(), `INSERT INTO comments(subject, subject_id, house_id, parent_id, author, body) VALUES (?,?,?,?,?,?)`,
		subject, id, h.ID, parent, str(m, "author"), str(m, "body"))
	if err != nil {
		fail(w, err)
		return
	}
	s.tellThread(r, subject, id, h, owner["house_id"].(int64), owner["label"].(string), snippet(str(m, "body"), 100))
	writeJSON(w, 201, map[string]any{"id": cid})
}

// tellThread pushes to the people the comment concerns: for an event, the house
// that called it plus everyone who answered yes or maybe; for a wish, the
// wisher plus every house that wants one.
func (s *Server) tellThread(r *http.Request, subject string, id int64, from *House, ownerHouse int64, label, body string) {
	kind, icon, url := "events", "💬", "#/tavern"
	q := `SELECT DISTINCT house_id FROM event_signups WHERE event_id=? AND state IN ('yes','maybe')`
	if subject == "wish" {
		kind, url = "tools", "#/shed"
		q = `SELECT DISTINCT house_id FROM wish_wants WHERE wish_id=?`
	}
	rows, _ := s.st.Rows(r.Context(), q, id)
	told := map[int64]bool{from.ID: true}
	for _, row := range append(rows, map[string]any{"house_id": ownerHouse}) {
		to := row["house_id"].(int64)
		if told[to] {
			continue
		}
		told[to] = true
		s.notifyHouse(kind, to, func(lang string) Payload {
			return Payload{Title: icon + " " + from.Name + tr(lang, " o: ", " on: ") + label, Body: body, URL: url}
		})
	}
}

func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, _ := pathID(r)
	row, err := s.st.One(r.Context(), `SELECT house_id FROM comments WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if row == nil {
		writeErr(w, 404, "not found")
		return
	}
	if row["house_id"].(int64) != h.ID && !h.IsSteward {
		writeErr(w, 403, "not yours")
		return
	}
	s.st.Exec(r.Context(), `DELETE FROM comments WHERE id=?`, id)
	w.WriteHeader(204)
}

// ---- options on a wish ----------------------------------------------------

// Anyone may add an option they found — a model, a price, a link. It is a
// finding, not a vote: options are never counted or ranked.
func (s *Server) createWishOption(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "bad id")
		return
	}
	m, err := readJSON(r)
	if err != nil || str(m, "text") == "" {
		writeErr(w, 400, "text required")
		return
	}
	wish, err := s.st.One(r.Context(), `SELECT house_id, text FROM wishes WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if wish == nil {
		writeErr(w, 404, "no such wish")
		return
	}
	url := strings.TrimSpace(str(m, "url"))
	if url != "" && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		url = "https://" + url
	}
	oid, err := s.st.Exec(r.Context(), `INSERT INTO wish_options(wish_id, house_id, text, url) VALUES (?,?,?,?)`, id, h.ID, str(m, "text"), url)
	if err != nil {
		fail(w, err)
		return
	}
	s.tellThread(r, "wish", id, h, wish["house_id"].(int64), wish["text"].(string), snippet(str(m, "text"), 100))
	writeJSON(w, 201, map[string]any{"id": oid})
}

func (s *Server) deleteWishOption(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, _ := pathID(r)
	row, err := s.st.One(r.Context(), `SELECT house_id FROM wish_options WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if row == nil {
		writeErr(w, 404, "not found")
		return
	}
	if row["house_id"].(int64) != h.ID && !h.IsSteward {
		writeErr(w, 403, "not yours")
		return
	}
	s.st.Exec(r.Context(), `DELETE FROM wish_options WHERE id=?`, id)
	w.WriteHeader(204)
}
