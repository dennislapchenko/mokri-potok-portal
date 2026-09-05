package httpapi

import (
	"net/http"
)

// Projects: a long job split into tasks, with events in the village calendar.
// Tasks are taken, never assigned. Done is a state, never a deletion.

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT p.*,`+houseJoin+`,
		(SELECT count(*) FROM project_tasks t WHERE t.project_id=p.id) AS tasks,
		(SELECT count(*) FROM project_tasks t WHERE t.project_id=p.id AND t.state='done') AS tasks_done,
		(SELECT count(*) FROM project_tasks t WHERE t.project_id=p.id AND t.state='open' AND t.assigned_to IS NULL) AS tasks_free,
		(SELECT min(e.starts_at) FROM events e WHERE e.project_id=p.id AND e.starts_at >= datetime('now')) AS next_event
		FROM projects p JOIN houses h ON h.id=p.house_id ORDER BY p.state, COALESCE(p.due_at, '9999'), p.created_at DESC`)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	id, _ := pathID(r)
	p, err := s.st.One(r.Context(), `SELECT p.*,`+houseJoin+` FROM projects p JOIN houses h ON h.id=p.house_id WHERE p.id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if p == nil {
		writeErr(w, 404, "no such project")
		return
	}
	tasks, err := s.st.Rows(r.Context(), `SELECT t.*, a.name AS assigned_name, a.crest AS assigned_crest FROM project_tasks t
		LEFT JOIN houses a ON a.id=t.assigned_to WHERE t.project_id=? ORDER BY t.state, COALESCE(t.due_at,'9999'), t.created_at`, id)
	if err != nil {
		fail(w, err)
		return
	}
	events, err := s.st.Rows(r.Context(), `SELECT e.*,`+houseJoin+`, pr.title AS project_title, tk.title AS task_title,
		(SELECT count(*) FROM event_signups s WHERE s.event_id=e.id) AS signups,
		(SELECT json_group_array(json_object('house_id', h2.id, 'name', h2.name, 'crest', h2.crest, 'note', s.note)) FROM event_signups s JOIN houses h2 ON h2.id=s.house_id WHERE s.event_id=e.id) AS signup_list,
		(SELECT count(*) FROM event_signups s WHERE s.event_id=e.id AND s.house_id=?) AS mine
		FROM events e JOIN houses h ON h.id=e.house_id LEFT JOIN projects pr ON pr.id=e.project_id LEFT JOIN project_tasks tk ON tk.id=e.task_id
		WHERE e.project_id=? ORDER BY e.starts_at`, houseFrom(r).ID, id)
	if err != nil {
		fail(w, err)
		return
	}
	p["tasks"] = tasks
	p["events"] = events
	writeJSON(w, 200, p)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	m, err := readJSON(r)
	if err != nil || str(m, "title") == "" {
		writeErr(w, 400, "title required")
		return
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO projects(house_id, title, notes, due_at) VALUES (?,?,?,?)`, h.ID, str(m, "title"), str(m, "notes"), nullIfEmpty(str(m, "due_at")))
	if err != nil {
		fail(w, err)
		return
	}
	s.notify("projects", h.ID, func(lang string) Payload {
		body := snippet(str(m, "notes"), 100)
		if d := str(m, "due_at"); d != "" {
			body = join(" · ", tr(lang, "do ", "by ")+humanWhen(d, lang, s.now()), body)
		}
		return Payload{Title: "📋 " + h.Name + tr(lang, " začenja: ", " starts: ") + str(m, "title"), Body: body, URL: "#/projects/" + itoa64(id)}
	})
	writeJSON(w, 201, map[string]any{"id": id})
}

// updateProject: title, notes, due_at, and state open/done (canEdit).
func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	id, _ := pathID(r)
	if !s.ownerOrSteward(w, r, "projects", id) {
		return
	}
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	for _, k := range []string{"title", "notes", "due_at"} {
		if _, ok := m[k]; ok {
			s.st.Exec(r.Context(), `UPDATE projects SET `+k+`=? WHERE id=?`, nullIfEmpty(str(m, k)), id)
		}
	}
	switch str(m, "state") {
	case "done":
		s.st.Exec(r.Context(), `UPDATE projects SET state='done', done_at=datetime('now') WHERE id=?`, id)
	case "open":
		s.st.Exec(r.Context(), `UPDATE projects SET state='open', done_at=NULL WHERE id=?`, id)
	}
	w.WriteHeader(204)
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	pid, _ := pathID(r)
	m, err := readJSON(r)
	if err != nil || str(m, "title") == "" {
		writeErr(w, 400, "title required")
		return
	}
	p, _ := s.st.One(r.Context(), `SELECT id FROM projects WHERE id=?`, pid)
	if p == nil {
		writeErr(w, 404, "no such project")
		return
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO project_tasks(project_id, house_id, title, notes, due_at) VALUES (?,?,?,?,?)`, pid, h.ID, str(m, "title"), str(m, "notes"), nullIfEmpty(str(m, "due_at")))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id})
}

// updateTask: any house takes a free task or lets it go; the holder, the
// creator or a steward closes it (with an optional closing note) or reopens it;
// creator or steward edits title, notes, due date.
func (s *Server) updateTask(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, _ := pathID(r)
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	t, err := s.st.One(r.Context(), `SELECT t.house_id, t.assigned_to, t.state, t.title, t.project_id, p.house_id AS project_house, p.title AS project_title FROM project_tasks t JOIN projects p ON p.id=t.project_id WHERE t.id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if t == nil {
		writeErr(w, 404, "no such task")
		return
	}
	holder, held := t["assigned_to"].(int64)
	creator := t["house_id"].(int64) == h.ID || t["project_house"].(int64) == h.ID || h.IsSteward
	if take, ok := m["take"].(bool); ok {
		if take {
			if held {
				writeErr(w, 409, "already taken")
				return
			}
			s.st.Exec(r.Context(), `UPDATE project_tasks SET assigned_to=? WHERE id=?`, h.ID, id)
			if owner := t["project_house"].(int64); owner != h.ID {
				title := t["title"].(string)
				s.notifyHouse("projects", owner, func(lang string) Payload {
					return Payload{Title: "📋 " + h.Name + tr(lang, " prevzame: ", " takes: ") + title, Body: t["project_title"].(string), URL: "#/projects/" + itoa64(t["project_id"].(int64))}
				})
			}
		} else {
			if !(held && holder == h.ID) && !creator {
				writeErr(w, 403, "not yours")
				return
			}
			s.st.Exec(r.Context(), `UPDATE project_tasks SET assigned_to=NULL WHERE id=?`, id)
		}
	}
	switch str(m, "state") {
	case "done":
		if !(held && holder == h.ID) && !creator {
			writeErr(w, 403, "not yours")
			return
		}
		s.st.Exec(r.Context(), `UPDATE project_tasks SET state='done', done_at=datetime('now'), closing_note=? WHERE id=?`, str(m, "closing_note"), id)
	case "open":
		if !(held && holder == h.ID) && !creator {
			writeErr(w, 403, "not yours")
			return
		}
		s.st.Exec(r.Context(), `UPDATE project_tasks SET state='open', done_at=NULL WHERE id=?`, id)
	}
	for _, k := range []string{"title", "notes", "due_at"} {
		if _, ok := m[k]; ok {
			if !creator {
				writeErr(w, 403, "not yours")
				return
			}
			s.st.Exec(r.Context(), `UPDATE project_tasks SET `+k+`=? WHERE id=?`, nullIfEmpty(str(m, k)), id)
		}
	}
	w.WriteHeader(204)
}

// deleteTask: the task's creator, the project's creator or a steward.
func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, _ := pathID(r)
	t, _ := s.st.One(r.Context(), `SELECT t.house_id, p.house_id AS project_house FROM project_tasks t JOIN projects p ON p.id=t.project_id WHERE t.id=?`, id)
	if t == nil {
		writeErr(w, 404, "no such task")
		return
	}
	if t["house_id"].(int64) != h.ID && t["project_house"].(int64) != h.ID && !h.IsSteward {
		writeErr(w, 403, "not yours")
		return
	}
	s.st.Exec(r.Context(), `DELETE FROM project_tasks WHERE id=?`, id)
	w.WriteHeader(204)
}

// ---- campground -----------------------------------------------------------

// The campground records that a house collected from a camper, and a note.
// No amounts, no plates: "grey camper", "family from NL". The cash box holds
// the money; this holds who has it until it is handed over.
func (s *Server) listCamp(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT x.*,`+houseJoin+` FROM camp_takings x JOIN houses h ON h.id=x.house_id ORDER BY x.taken_on DESC, x.created_at DESC LIMIT 500`)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) createCamp(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	m, err := readJSON(r)
	if err != nil || str(m, "from_who") == "" || str(m, "taken_on") == "" {
		writeErr(w, 400, "from_who and taken_on required")
		return
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO camp_takings(house_id, collected_by, from_who, taken_on, notes) VALUES (?,?,?,?,?)`,
		h.ID, str(m, "collected_by"), str(m, "from_who"), str(m, "taken_on"), str(m, "notes"))
	if err != nil {
		fail(w, err)
		return
	}
	s.notify("camp", h.ID, func(lang string) Payload {
		who := h.Name
		if c := str(m, "collected_by"); c != "" {
			who = c + " · " + h.Name
		}
		return Payload{Title: "🏕️ " + who + tr(lang, " je pobral kamp", " collected at the camp"), Body: snippet(join(" — ", str(m, "from_who"), str(m, "notes")), 120), URL: "#/camp"}
	})
	writeJSON(w, 201, map[string]any{"id": id})
}

func (s *Server) updateCamp(w http.ResponseWriter, r *http.Request) {
	id, _ := pathID(r)
	if !s.ownerOrSteward(w, r, "camp_takings", id) {
		return
	}
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	switch str(m, "state") {
	case "handed":
		s.st.Exec(r.Context(), `UPDATE camp_takings SET state='handed', handed_at=datetime('now') WHERE id=?`, id)
	case "held":
		s.st.Exec(r.Context(), `UPDATE camp_takings SET state='held', handed_at=NULL WHERE id=?`, id)
	}
	if n, ok := m["notes"]; ok {
		s.st.Exec(r.Context(), `UPDATE camp_takings SET notes=? WHERE id=?`, n, id)
	}
	w.WriteHeader(204)
}
