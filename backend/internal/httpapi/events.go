package httpapi

import (
	"net/http"
)

// Event sign-ups and comments.
//
// A sign-up is an answer: yes, no or maybe. A confirmed "not coming" tells the
// village more than silence, so it is stored, not implied by absence.
//
// Comments are the event's own little board: one reply level, same shape as the
// tavern. They replaced the single note a sign-up used to carry.

var signupStates = []string{"yes", "no", "maybe"}

func (s *Server) listComments(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "bad id")
		return
	}
	rows, err := s.st.Rows(r.Context(), `SELECT c.*,`+houseJoin+` FROM event_comments c JOIN houses h ON h.id=c.house_id
		WHERE c.event_id=? ORDER BY c.created_at LIMIT 500`, id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) createComment(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "bad id")
		return
	}
	m, err := readJSON(r)
	if err != nil || str(m, "body") == "" {
		writeErr(w, 400, "body required")
		return
	}
	ev, err := s.st.One(r.Context(), `SELECT house_id, title FROM events WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if ev == nil {
		writeErr(w, 404, "no such event")
		return
	}
	// A reply must belong to the same event, or a comment could be hung on
	// another event's thread.
	var parent any
	if v, ok := m["parent_id"].(float64); ok && v > 0 {
		p, _ := s.st.One(r.Context(), `SELECT event_id, parent_id FROM event_comments WHERE id=?`, int64(v))
		if p == nil || p["event_id"].(int64) != id {
			writeErr(w, 400, "no such comment on this event")
			return
		}
		// One level only: a reply to a reply hangs off the root.
		if p["parent_id"] != nil {
			parent = p["parent_id"]
		} else {
			parent = int64(v)
		}
	}
	cid, err := s.st.Exec(r.Context(), `INSERT INTO event_comments(event_id, house_id, parent_id, author, body) VALUES (?,?,?,?,?)`,
		id, h.ID, parent, str(m, "author"), str(m, "body"))
	if err != nil {
		fail(w, err)
		return
	}
	// The audience is the house that called it plus everyone who answered yes or
	// maybe — they are the ones the comment is about.
	title, body := ev["title"].(string), snippet(str(m, "body"), 100)
	told := map[int64]bool{h.ID: true}
	rows, _ := s.st.Rows(r.Context(), `SELECT DISTINCT house_id FROM event_signups WHERE event_id=? AND state IN ('yes','maybe')`, id)
	for _, row := range append(rows, map[string]any{"house_id": ev["house_id"]}) {
		to := row["house_id"].(int64)
		if told[to] {
			continue
		}
		told[to] = true
		s.notifyHouse("events", to, func(lang string) Payload {
			return Payload{Title: "💬 " + h.Name + tr(lang, " o dogodku: ", " on: ") + title, Body: body, URL: "#/tavern"}
		})
	}
	writeJSON(w, 201, map[string]any{"id": cid})
}

func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, _ := pathID(r)
	row, err := s.st.One(r.Context(), `SELECT house_id FROM event_comments WHERE id=?`, id)
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
	s.st.Exec(r.Context(), `DELETE FROM event_comments WHERE id=?`, id)
	w.WriteHeader(204)
}
