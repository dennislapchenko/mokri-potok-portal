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
		(SELECT count(*) FROM event_signups s WHERE s.event_id=e.id AND s.state='yes' AND s.answered_version >= e.time_version) AS signups,
		(SELECT json_group_array(json_object('house_id', h2.id, 'name', h2.name, 'crest', h2.crest, 'note', s.note, 'state', s.state, 'stale', CASE WHEN s.answered_version < e.time_version THEN 1 ELSE 0 END)) FROM event_signups s JOIN houses h2 ON h2.id=s.house_id WHERE s.event_id=e.id) AS signup_list,
		(SELECT s.state FROM event_signups s WHERE s.event_id=e.id AND s.house_id=?) AS mine,
		(SELECT count(*) FROM comments c WHERE c.subject='event' AND c.subject_id=e.id) AS comments,
		ed.name AS edited_by_name
		FROM events e JOIN houses h ON h.id=e.house_id LEFT JOIN projects pr ON pr.id=e.project_id LEFT JOIN project_tasks tk ON tk.id=e.task_id LEFT JOIN houses ed ON ed.id=e.edited_by
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

// updateTask: any house takes a free task; the holder lets it go. The task's
// creator, the project's creator or a steward may also assign a house
// (things get agreed in real life) or clear the holder, close with a note,
// reopen, and edit title, notes, due date. The assigned house is told.
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
	// Permission checks first, writes after: a mixed body must not half-apply.
	for _, k := range []string{"title", "notes", "due_at"} {
		if _, ok := m[k]; ok && !creator {
			writeErr(w, 403, "not yours")
			return
		}
	}
	if st := str(m, "state"); (st == "done" || st == "open") && !(held && holder == h.ID) && !creator {
		writeErr(w, 403, "not yours")
		return
	}
	if take, ok := m["take"].(bool); ok {
		if take {
			if held {
				writeErr(w, 409, "already taken")
				return
			}
			if t["state"] != "open" {
				writeErr(w, 409, "task is done")
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
	// Assign: creator only, to a house that exists; the house hears about it.
	if v, ok := m["assigned_to"].(float64); ok {
		if !creator {
			writeErr(w, 403, "only the creator assigns")
			return
		}
		to := int64(v)
		hs, _ := s.st.One(r.Context(), `SELECT name FROM houses WHERE id=?`, to)
		if hs == nil {
			writeErr(w, 404, "no such house")
			return
		}
		s.st.Exec(r.Context(), `UPDATE project_tasks SET assigned_to=? WHERE id=?`, to, id)
		if to != h.ID {
			title, ptitle := t["title"].(string), t["project_title"].(string)
			s.notifyHouse("projects", to, func(lang string) Payload {
				return Payload{Title: "📋 " + h.Name + tr(lang, " vam predaja: ", " hands you: ") + title, Body: ptitle, URL: "#/projects/" + itoa64(t["project_id"].(int64))}
			})
		}
	}
	switch str(m, "state") {
	case "done":
		s.st.Exec(r.Context(), `UPDATE project_tasks SET state='done', done_at=datetime('now'), closing_note=? WHERE id=?`, str(m, "closing_note"), id)
	case "open":
		s.st.Exec(r.Context(), `UPDATE project_tasks SET state='open', done_at=NULL WHERE id=?`, id)
	}
	for _, k := range []string{"title", "notes", "due_at"} {
		if _, ok := m[k]; ok {
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

// A camper's stay is three states: arrived (a house noticed), held (a house
// has the money), handed (it reached the box). No amounts, no plates. A row is
// who noticed, who holds, and a note.
func (s *Server) listCamp(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT x.*,`+houseJoin+`, b.name AS held_by_name, b.crest AS held_by_crest
		FROM camp_takings x JOIN houses h ON h.id=x.house_id LEFT JOIN houses b ON b.id=x.held_by
		ORDER BY CASE x.state WHEN 'arrived' THEN 0 WHEN 'held' THEN 1 ELSE 2 END, x.taken_on DESC, x.created_at DESC LIMIT 500`)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

// createCamp: "a camper arrived" with an optional note. With have_money the
// noticing house already holds the cash and the row lands as handed — the
// owner's choice for now, to be revisited.
func (s *Server) createCamp(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	on := str(m, "taken_on")
	if on == "" {
		on = s.now().Format("2006-01-02")
	}
	have, _ := m["have_money"].(bool)
	state, heldBy, heldAt, handedAt := "arrived", any(nil), any(nil), any(nil)
	if have {
		state, heldBy, heldAt, handedAt = "handed", h.ID, "now", "now"
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO camp_takings(house_id, collected_by, from_who, taken_on, notes, state, held_by,
		held_at, handed_at) VALUES (?,?,?,?,?,?,?, CASE WHEN ? IS NULL THEN NULL ELSE datetime('now') END, CASE WHEN ? IS NULL THEN NULL ELSE datetime('now') END)`,
		h.ID, str(m, "collected_by"), str(m, "from_who"), on, str(m, "notes"), state, heldBy, heldAt, handedAt)
	if err != nil {
		fail(w, err)
		return
	}
	s.notify("camp", h.ID, func(lang string) Payload {
		title := "🏕️ " + h.Name + tr(lang, ": kamper je prišel", ": a camper arrived")
		if have {
			title = "🏕️ " + h.Name + tr(lang, " je pobral kamp", " collected at the camp")
		}
		return Payload{Title: title, Body: snippet(str(m, "notes"), 80), URL: "#/camp"}
	})
	writeJSON(w, 201, map[string]any{"id": id})
}

// updateCamp: claim (any house, from arrived), handed (holder or steward),
// back to held (holder or steward), notes (noticer, holder or steward).
func (s *Server) updateCamp(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, _ := pathID(r)
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	row, err := s.st.One(r.Context(), `SELECT house_id, held_by, state, notes FROM camp_takings WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if row == nil {
		writeErr(w, 404, "not found")
		return
	}
	holder, held := row["held_by"].(int64)
	mayEdit := row["house_id"].(int64) == h.ID || (held && holder == h.ID) || h.IsSteward
	if claim, _ := m["claim"].(bool); claim {
		if row["state"] != "arrived" {
			writeErr(w, 409, "money already claimed")
			return
		}
		s.st.Exec(r.Context(), `UPDATE camp_takings SET state='held', held_by=?, held_at=datetime('now') WHERE id=?`, h.ID, id)
		if n := str(m, "notes"); n != "" && row["notes"] == "" {
			s.st.Exec(r.Context(), `UPDATE camp_takings SET notes=? WHERE id=?`, n, id)
		}
		s.notify("camp", h.ID, func(lang string) Payload {
			return Payload{Title: "💰 " + h.Name + tr(lang, " ima denar od kampa", " has the camp money"), Body: snippet(str(m, "notes"), 80), URL: "#/camp"}
		})
		w.WriteHeader(204)
		return
	}
	switch str(m, "state") {
	case "handed":
		if row["state"] != "held" {
			writeErr(w, 409, "nobody holds the money yet")
			return
		}
		if holder != h.ID && !h.IsSteward {
			writeErr(w, 403, "only the house holding it hands it over")
			return
		}
		s.st.Exec(r.Context(), `UPDATE camp_takings SET state='handed', handed_at=datetime('now') WHERE id=?`, id)
	case "held":
		if !(held && holder == h.ID) && !h.IsSteward {
			writeErr(w, 403, "not yours")
			return
		}
		s.st.Exec(r.Context(), `UPDATE camp_takings SET state='held', handed_at=NULL WHERE id=?`, id)
	}
	if n, ok := m["notes"]; ok && str(m, "claim") == "" {
		if !mayEdit {
			writeErr(w, 403, "not yours")
			return
		}
		s.st.Exec(r.Context(), `UPDATE camp_takings SET notes=? WHERE id=?`, n, id)
	}
	w.WriteHeader(204)
}
