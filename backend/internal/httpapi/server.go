// Package httpapi is the HTTP surface: stdlib mux (Go 1.22 patterns), JSON in
// and out, bearer-token auth (auth.go). Every resource follows one shape:
// GET list, POST create, PUT /{id} partial update, DELETE /{id}. A house may
// edit its own rows; a steward may edit any row; "taken / claimed / watcher"
// fields are the one thing any house may set on another house's row.
package httpapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/config"
	"github.com/dennislapchenko/mokri-potok-portal/backend/internal/store"
)

type Server struct {
	st   *store.Store
	cfg  config.Config
	mux  *http.ServeMux
	send Sender           // nil until first use; tests inject a fake
	now  func() time.Time // swapped in tests so "today"/"tomorrow" are deterministic

	tries  *limiter // per-IP cap on code guessing
	misses *counter // village-wide wrong-code counter, burns live pairing codes
}

func New(st *store.Store, cfg config.Config) *Server {
	s := &Server{st: st, cfg: cfg, mux: http.NewServeMux(), now: time.Now, tries: newLimiter(), misses: &counter{}}
	s.routes()
	return s
}

func (s *Server) Handler() http.Handler {
	return cors(s.cfg.CORSOrigins, logRequests(s.mux))
}

func (s *Server) routes() {
	m := s.mux
	m.HandleFunc("GET /api/healthz", s.healthz)
	m.HandleFunc("GET /api/status", s.status)
	m.HandleFunc("POST /api/bootstrap", s.bootstrap)
	m.HandleFunc("POST /api/join", s.join)
	m.HandleFunc("POST /api/pair", s.requireHouse(s.createPairing))

	m.HandleFunc("GET /api/me", s.requireHouse(s.me))
	m.HandleFunc("GET /api/devices", s.requireHouse(s.listDevices))
	m.HandleFunc("DELETE /api/devices/{id}", s.requireHouse(s.deleteDevice))

	m.HandleFunc("GET /api/houses", s.requireHouse(s.listHouses))
	m.HandleFunc("POST /api/houses", s.requireSteward(s.createHouse))
	m.HandleFunc("PUT /api/houses/{id}", s.requireHouse(s.updateHouse))
	m.HandleFunc("DELETE /api/houses/{id}", s.requireSteward(s.deleteHouse))
	m.HandleFunc("GET /api/houses/{id}/invite", s.requireSteward(s.getInvite))
	m.HandleFunc("POST /api/houses/{id}/invite", s.requireSteward(s.rotateInvite))

	// Tavern
	m.HandleFunc("GET /api/posts", s.requireHouse(s.listPosts))
	m.HandleFunc("POST /api/posts", s.requireHouse(s.createPost))
	m.HandleFunc("PUT /api/posts/{id}", s.requireHouse(s.updatePost))
	m.HandleFunc("DELETE /api/posts/{id}", s.requireHouse(s.deleteRow("posts")))
	// Bell tower
	m.HandleFunc("GET /api/events", s.requireHouse(s.listEvents))
	m.HandleFunc("POST /api/events", s.requireHouse(s.createEvent))
	m.HandleFunc("PUT /api/events/{id}", s.requireHouse(s.updateEvent))
	m.HandleFunc("DELETE /api/events/{id}", s.requireHouse(s.deleteRow("events")))
	m.HandleFunc("POST /api/events/{id}/signup", s.requireHouse(s.signUp))
	m.HandleFunc("DELETE /api/events/{id}/signup", s.requireHouse(s.signOff))
	// Tool shed
	m.HandleFunc("GET /api/tools", s.requireHouse(s.listTools))
	m.HandleFunc("POST /api/tools", s.requireHouse(s.createTool))
	m.HandleFunc("PUT /api/tools/{id}", s.requireHouse(s.updateTool))
	m.HandleFunc("DELETE /api/tools/{id}", s.requireHouse(s.deleteRow("tools")))
	m.HandleFunc("GET /api/tools/{id}/photo", s.requireHouse(s.getToolPhoto))
	m.HandleFunc("PUT /api/tools/{id}/photo", s.requireHouse(s.putToolPhoto))
	m.HandleFunc("DELETE /api/tools/{id}/photo", s.requireHouse(s.deleteToolPhoto))
	m.HandleFunc("GET /api/wishes", s.requireHouse(s.listWishes))
	m.HandleFunc("POST /api/wishes", s.requireHouse(s.createWish))
	m.HandleFunc("PUT /api/wishes/{id}", s.requireHouse(s.updateWish))
	m.HandleFunc("DELETE /api/wishes/{id}", s.requireHouse(s.deleteRow("wishes")))
	// Market
	m.HandleFunc("GET /api/runs", s.requireHouse(s.listRuns))
	m.HandleFunc("POST /api/runs", s.requireHouse(s.createRun))
	m.HandleFunc("DELETE /api/runs/{id}", s.requireHouse(s.deleteRow("runs")))
	m.HandleFunc("GET /api/needs", s.requireHouse(s.listNeeds))
	m.HandleFunc("POST /api/needs", s.requireHouse(s.createNeed))
	m.HandleFunc("PUT /api/needs/{id}", s.requireHouse(s.updateNeed))
	m.HandleFunc("DELETE /api/needs/{id}", s.requireHouse(s.deleteRow("needs")))
	m.HandleFunc("GET /api/offers", s.requireHouse(s.listOffers))
	m.HandleFunc("POST /api/offers", s.requireHouse(s.createOffer))
	m.HandleFunc("PUT /api/offers/{id}", s.requireHouse(s.updateOffer))
	m.HandleFunc("DELETE /api/offers/{id}", s.requireHouse(s.deleteRow("offers")))
	// Watchtower
	m.HandleFunc("GET /api/away", s.requireHouse(s.listAway))
	m.HandleFunc("POST /api/away", s.requireHouse(s.createAway))
	m.HandleFunc("PUT /api/away/{id}", s.requireHouse(s.updateAway))
	m.HandleFunc("DELETE /api/away/{id}", s.requireHouse(s.deleteRow("away")))
	// Web push
	m.HandleFunc("GET /api/push/key", s.pushKey)
	m.HandleFunc("POST /api/push/subscribe", s.requireHouse(s.subscribe))
	m.HandleFunc("DELETE /api/push/subscribe", s.requireHouse(s.unsubscribe))
	m.HandleFunc("GET /api/me/prefs", s.requireHouse(s.getPrefs))
	m.HandleFunc("PUT /api/me/prefs", s.requireHouse(s.putPrefs))
	m.HandleFunc("PUT /api/prefs/global", s.requireSteward(s.putGlobalPrefs))
	// Projects
	m.HandleFunc("GET /api/projects", s.requireHouse(s.listProjects))
	m.HandleFunc("POST /api/projects", s.requireHouse(s.createProject))
	m.HandleFunc("GET /api/projects/{id}", s.requireHouse(s.getProject))
	m.HandleFunc("PUT /api/projects/{id}", s.requireHouse(s.updateProject))
	m.HandleFunc("DELETE /api/projects/{id}", s.requireHouse(s.deleteRow("projects")))
	m.HandleFunc("POST /api/projects/{id}/tasks", s.requireHouse(s.createTask))
	m.HandleFunc("PUT /api/tasks/{id}", s.requireHouse(s.updateTask))
	m.HandleFunc("DELETE /api/tasks/{id}", s.requireHouse(s.deleteTask))
	// Campground
	m.HandleFunc("GET /api/camp", s.requireHouse(s.listCamp))
	m.HandleFunc("POST /api/camp", s.requireHouse(s.createCamp))
	m.HandleFunc("PUT /api/camp/{id}", s.requireHouse(s.updateCamp))
	m.HandleFunc("DELETE /api/camp/{id}", s.requireHouse(s.deleteRow("camp_takings")))
	// Exit path: everything as one JSON document (steward only).
	m.HandleFunc("GET /api/export", s.requireSteward(s.export))
}

// ---- plumbing ------------------------------------------------------------

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// readJSON decodes a body of at most 64 KB into a map; strings are trimmed.
func readJSON(r *http.Request) (map[string]any, error) {
	var m map[string]any
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 64<<10))
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	for k, v := range m {
		if s, ok := v.(string); ok {
			m[k] = strings.TrimSpace(s)
		}
	}
	return m, nil
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

func fail(w http.ResponseWriter, err error) {
	log.Printf("error: %v", err)
	writeErr(w, 500, "server error")
}

// ownerOrSteward checks that the row in `table` belongs to the caller, or the
// caller is a steward. Returns false after writing the error response.
func (s *Server) ownerOrSteward(w http.ResponseWriter, r *http.Request, table string, id int64) bool {
	h := houseFrom(r)
	row, err := s.st.One(r.Context(), `SELECT house_id FROM `+table+` WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return false
	}
	if row == nil {
		writeErr(w, 404, "not found")
		return false
	}
	if row["house_id"].(int64) != h.ID && !h.IsSteward {
		writeErr(w, 403, "not yours")
		return false
	}
	return true
}

func (s *Server) deleteRow(table string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			writeErr(w, 400, "bad id")
			return
		}
		if !s.ownerOrSteward(w, r, table, id) {
			return
		}
		if _, err := s.st.Exec(r.Context(), `DELETE FROM `+table+` WHERE id=?`, id); err != nil {
			fail(w, err)
			return
		}
		w.WriteHeader(204)
	}
}

func cors(allowed []string, next http.Handler) http.Handler {
	allow := map[string]bool{}
	for _, o := range allowed {
		allow[o] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if o := r.Header.Get("Origin"); o != "" && allow[o] {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", o)
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		t := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(t).Round(time.Millisecond))
	})
}

// ---- health, bootstrap, join -------------------------------------------

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	if err := s.st.Ping(r.Context()); err != nil {
		writeErr(w, 503, "db down")
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// status tells an unauthenticated SPA whether the village exists yet.
func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	row, err := s.st.One(r.Context(), `SELECT count(*) AS n FROM houses`)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"houses": row["n"], "bootstrap_needed": row["n"].(int64) == 0})
}

// BootstrapCode returns the code that creates the first steward house. Fixed
// from env when set, otherwise generated once and kept in settings.
func (s *Server) BootstrapCode() (string, error) {
	if s.cfg.BootstrapCode != "" {
		return s.cfg.BootstrapCode, nil
	}
	row, err := s.st.One(nil2(), `SELECT value FROM settings WHERE key='bootstrap_code'`)
	if err != nil {
		return "", err
	}
	if row != nil {
		return row["value"].(string), nil
	}
	code := inviteCode()
	_, err = s.st.Exec(nil2(), `INSERT INTO settings(key,value) VALUES ('bootstrap_code',?)`, code)
	return code, err
}

// bootstrap creates the first steward house. Refused once any house exists.
func (s *Server) bootstrap(w http.ResponseWriter, r *http.Request) {
	if !s.tries.allow(clientIP(r), 12, 10*time.Minute) {
		writeErr(w, http.StatusTooManyRequests, "too many attempts, wait a few minutes")
		return
	}
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	row, _ := s.st.One(r.Context(), `SELECT count(*) AS n FROM houses`)
	if row["n"].(int64) > 0 {
		writeErr(w, 409, "village already exists")
		return
	}
	code, err := s.BootstrapCode()
	if err != nil {
		fail(w, err)
		return
	}
	if str(m, "code") != code {
		writeErr(w, 403, "wrong code")
		return
	}
	name := str(m, "name")
	if name == "" {
		name = "Oskrbnik"
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO houses(name, crest, is_steward) VALUES (?,?,1)`, name, "🗝️")
	if err != nil {
		fail(w, err)
		return
	}
	tok, err := s.newDevice(r.Context(), id, str(m, "device"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"token": tok, "house_id": id})
}

// join turns an invite code into a device token. The code stays valid until
// it expires so every member of the house can use the same link.
func (s *Server) join(w http.ResponseWriter, r *http.Request) {
	// A pairing code is only six digits, so guessing has to cost something.
	if !s.tries.allow(clientIP(r), 12, 10*time.Minute) {
		writeErr(w, http.StatusTooManyRequests, "too many attempts, wait a few minutes")
		return
	}
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	code := strings.ToLower(strings.TrimSpace(str(m, "code")))
	// A code is either a steward's invite (a house joins) or a house's own
	// pairing code (one more phone of a house already inside).
	inv, err := s.st.One(r.Context(), `SELECT house_id, expires_at, 'invite' AS src FROM invites WHERE code=?
		UNION ALL SELECT house_id, expires_at, 'pairing' FROM pairings WHERE code=? AND used_at IS NULL`, code, code)
	if err != nil {
		fail(w, err)
		return
	}
	if inv == nil {
		// A wrong six-digit guess also spends the live pairing code's budget:
		// five misses anywhere in the village and every live code is dropped.
		if s.misses.count(1) >= 5 {
			s.st.Exec(r.Context(), `DELETE FROM pairings WHERE used_at IS NULL`)
			s.misses.reset()
		}
		writeErr(w, 404, "unknown code")
		return
	}
	if exp, _ := time.Parse(time.RFC3339, inv["expires_at"].(string)); time.Now().After(exp) {
		writeErr(w, 410, "code expired")
		return
	}
	if inv["src"] == "pairing" {
		// Single use: burn it before minting the device.
		res, err := s.st.Exec(r.Context(), `UPDATE pairings SET used_at=datetime('now') WHERE code=? AND used_at IS NULL`, code)
		if err != nil {
			fail(w, err)
			return
		}
		_ = res
	}
	tok, err := s.newDevice(r.Context(), inv["house_id"].(int64), str(m, "device"))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"token": tok, "house_id": inv["house_id"]})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	row, err := s.st.One(r.Context(), `SELECT id, name, crest, color, kind, is_steward FROM houses WHERE id=?`, h.ID)
	if err != nil {
		fail(w, err)
		return
	}
	row["device_id"] = h.DeviceID
	writeJSON(w, 200, row)
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT id, label, created_at, last_seen FROM devices WHERE house_id=? ORDER BY last_seen DESC`, houseFrom(r).ID)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	id, _ := pathID(r)
	if _, err := s.st.Exec(r.Context(), `DELETE FROM devices WHERE id=? AND house_id=?`, id, houseFrom(r).ID); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(204)
}

// ---- houses --------------------------------------------------------------

func (s *Server) listHouses(w http.ResponseWriter, r *http.Request) {
	houses, err := s.st.Rows(r.Context(), `SELECT id, name, crest, color, kind, is_steward FROM houses ORDER BY kind, name`)
	if err != nil {
		fail(w, err)
		return
	}
	parcels, err := s.st.Rows(r.Context(), `SELECT house_id, parcel FROM house_parcels`)
	if err != nil {
		fail(w, err)
		return
	}
	byHouse := map[int64][]string{}
	for _, p := range parcels {
		byHouse[p["house_id"].(int64)] = append(byHouse[p["house_id"].(int64)], p["parcel"].(string))
	}
	for _, h := range houses {
		ps := byHouse[h["id"].(int64)]
		if ps == nil {
			ps = []string{}
		}
		h["parcels"] = ps
	}
	writeJSON(w, 200, houses)
}

func (s *Server) createHouse(w http.ResponseWriter, r *http.Request) {
	m, err := readJSON(r)
	if err != nil || str(m, "name") == "" {
		writeErr(w, 400, "name required")
		return
	}
	crest, color, kind := str(m, "crest"), str(m, "color"), str(m, "kind")
	if crest == "" {
		crest = "🏠"
	}
	if color == "" {
		color = "#b5651d"
	}
	if kind != "common" {
		kind = "house"
	}
	steward := 0
	if b, _ := m["is_steward"].(bool); b {
		steward = 1
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO houses(name, crest, color, kind, is_steward) VALUES (?,?,?,?,?)`, str(m, "name"), crest, color, kind, steward)
	if err != nil {
		fail(w, err)
		return
	}
	if kind == "common" {
		// A common place is land on the map, not an account: no invite.
		writeJSON(w, 201, map[string]any{"id": id})
		return
	}
	inv, err := s.newInvite(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "invite": inv})
}

// updateHouse: a house edits its own name/crest/color; stewards also edit
// kind, is_steward and the parcel list (replaced wholesale).
func (s *Server) updateHouse(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "bad id")
		return
	}
	if id != h.ID && !h.IsSteward {
		writeErr(w, 403, "not yours")
		return
	}
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	for _, k := range []string{"name", "crest", "color"} {
		if v := str(m, k); v != "" {
			if _, err := s.st.Exec(r.Context(), `UPDATE houses SET `+k+`=? WHERE id=?`, v, id); err != nil {
				fail(w, err)
				return
			}
		}
	}
	if h.IsSteward {
		if v := str(m, "kind"); v == "house" || v == "common" {
			s.st.Exec(r.Context(), `UPDATE houses SET kind=? WHERE id=?`, v, id)
		}
		if b, ok := m["is_steward"].(bool); ok && id != h.ID {
			st := 0
			if b {
				st = 1
			}
			s.st.Exec(r.Context(), `UPDATE houses SET is_steward=? WHERE id=?`, st, id)
		}
		if ps, ok := m["parcels"].([]any); ok {
			s.st.Exec(r.Context(), `DELETE FROM house_parcels WHERE house_id=?`, id)
			for _, p := range ps {
				if ps, ok := p.(string); ok && ps != "" {
					// A parcel can belong to one house; the newest assignment wins.
					s.st.Exec(r.Context(), `INSERT OR REPLACE INTO house_parcels(house_id, parcel) VALUES (?,?)`, id, ps)
				}
			}
		}
	}
	w.WriteHeader(204)
}

func (s *Server) deleteHouse(w http.ResponseWriter, r *http.Request) {
	id, _ := pathID(r)
	if id == houseFrom(r).ID {
		writeErr(w, 400, "cannot delete your own house")
		return
	}
	if _, err := s.st.Exec(r.Context(), `DELETE FROM houses WHERE id=?`, id); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(204)
}

func (s *Server) getInvite(w http.ResponseWriter, r *http.Request) {
	id, _ := pathID(r)
	row, err := s.st.One(r.Context(), `SELECT code, expires_at FROM invites WHERE house_id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if row == nil {
		writeJSON(w, 200, map[string]any{})
		return
	}
	writeJSON(w, 200, row)
}

func (s *Server) rotateInvite(w http.ResponseWriter, r *http.Request) {
	id, _ := pathID(r)
	inv, err := s.newInvite(r.Context(), id)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 201, inv)
}

// ---- tavern --------------------------------------------------------------

const houseJoin = ` h.name AS house_name, h.crest AS house_crest, h.color AS house_color `

func (s *Server) listPosts(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT p.*,`+houseJoin+`FROM posts p JOIN houses h ON h.id=p.house_id ORDER BY p.pinned DESC, p.created_at DESC LIMIT 500`)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	m, err := readJSON(r)
	if err != nil || str(m, "body") == "" {
		writeErr(w, 400, "body required")
		return
	}
	var parent any
	if p, ok := m["parent_id"].(float64); ok {
		parent = int64(p)
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO posts(house_id, parent_id, author, body) VALUES (?,?,?,?)`, houseFrom(r).ID, parent, str(m, "author"), str(m, "body"))
	if err != nil {
		fail(w, err)
		return
	}
	who := houseFrom(r).Name
	if a := str(m, "author"); a != "" {
		who = a + " · " + who
	}
	s.notify("posts", houseFrom(r).ID, func(lang string) Payload {
		return Payload{Title: "🍺 " + who + tr(lang, " v gostilni", " in the tavern"), Body: snippet(str(m, "body"), 140), URL: "#/tavern?at=board"}
	})
	writeJSON(w, 201, map[string]any{"id": id})
}

func (s *Server) updatePost(w http.ResponseWriter, r *http.Request) {
	id, _ := pathID(r)
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	if pin, ok := m["pinned"].(bool); ok {
		if !houseFrom(r).IsSteward {
			writeErr(w, 403, "stewards pin")
			return
		}
		p := 0
		if pin {
			p = 1
		}
		s.st.Exec(r.Context(), `UPDATE posts SET pinned=? WHERE id=?`, p, id)
	}
	if b := str(m, "body"); b != "" {
		if !s.ownerOrSteward(w, r, "posts", id) {
			return
		}
		s.st.Exec(r.Context(), `UPDATE posts SET body=? WHERE id=?`, b, id)
	}
	w.WriteHeader(204)
}

// ---- bell tower ----------------------------------------------------------

func (s *Server) listEvents(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT e.*,`+houseJoin+`, pr.title AS project_title, tk.title AS task_title,
		(SELECT count(*) FROM event_signups s WHERE s.event_id=e.id) AS signups,
		(SELECT json_group_array(json_object('house_id', h2.id, 'name', h2.name, 'crest', h2.crest, 'note', s.note)) FROM event_signups s JOIN houses h2 ON h2.id=s.house_id WHERE s.event_id=e.id) AS signup_list,
		(SELECT count(*) FROM event_signups s WHERE s.event_id=e.id AND s.house_id=?) AS mine
		FROM events e JOIN houses h ON h.id=e.house_id LEFT JOIN projects pr ON pr.id=e.project_id LEFT JOIN project_tasks tk ON tk.id=e.task_id
		WHERE e.starts_at >= date('now','-60 days') ORDER BY e.starts_at LIMIT 500`, houseFrom(r).ID)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) createEvent(w http.ResponseWriter, r *http.Request) {
	m, err := readJSON(r)
	if err != nil || str(m, "title") == "" || str(m, "starts_at") == "" {
		writeErr(w, 400, "title and starts_at required")
		return
	}
	kind := str(m, "kind")
	if kind != "work" && kind != "alarm" {
		kind = "event"
	}
	// An event may belong to a project, and to one of that project's tasks.
	// The task decides the project; a task from another project is refused.
	var projectID, taskID any
	if v, ok := m["project_id"].(float64); ok && v > 0 {
		projectID = int64(v)
	}
	if v, ok := m["task_id"].(float64); ok && v > 0 {
		tk, _ := s.st.One(r.Context(), `SELECT project_id FROM project_tasks WHERE id=?`, int64(v))
		if tk == nil || (projectID != nil && tk["project_id"].(int64) != projectID.(int64)) {
			writeErr(w, 400, "task does not belong to that project")
			return
		}
		taskID, projectID = int64(v), tk["project_id"]
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO events(house_id, title, kind, starts_at, ends_at, place, notes, project_id, task_id) VALUES (?,?,?,?,?,?,?,?,?)`,
		houseFrom(r).ID, str(m, "title"), kind, str(m, "starts_at"), nullIfEmpty(str(m, "ends_at")), str(m, "place"), str(m, "notes"), projectID, taskID)
	if err != nil {
		fail(w, err)
		return
	}
	icon := map[string]string{"event": "🔔", "work": "🤝", "alarm": "🚨"}[kind]
	// An alarm is the one thing that may wake the village at night.
	ring := s.notify
	if kind == "alarm" {
		ring = s.notifyUrgent
	}
	ring("events", houseFrom(r).ID, func(lang string) Payload {
		body := join(" · ", humanWhen(str(m, "starts_at"), lang, s.now()), str(m, "place"))
		if n := str(m, "notes"); n != "" {
			body = join(" — ", body, snippet(n, 70))
		}
		return Payload{Title: icon + " " + str(m, "title"), Body: body, URL: "#/bell"}
	})
	writeJSON(w, 201, map[string]any{"id": id})
}

func (s *Server) updateEvent(w http.ResponseWriter, r *http.Request) {
	id, _ := pathID(r)
	if !s.ownerOrSteward(w, r, "events", id) {
		return
	}
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	for _, k := range []string{"title", "kind", "starts_at", "ends_at", "place", "notes"} {
		if _, ok := m[k]; ok {
			s.st.Exec(r.Context(), `UPDATE events SET `+k+`=? WHERE id=?`, str(m, k), id)
		}
	}
	w.WriteHeader(204)
}

// ---- market --------------------------------------------------------------

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT x.*,`+houseJoin+`FROM runs x JOIN houses h ON h.id=x.house_id
		WHERE x.cutoff_at >= datetime('now','-1 day') ORDER BY x.cutoff_at`)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	m, err := readJSON(r)
	if err != nil || str(m, "destination") == "" || str(m, "cutoff_at") == "" {
		writeErr(w, 400, "destination and cutoff_at required")
		return
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO runs(house_id, destination, cutoff_at, notes) VALUES (?,?,?,?)`, houseFrom(r).ID, str(m, "destination"), str(m, "cutoff_at"), str(m, "notes"))
	if err != nil {
		fail(w, err)
		return
	}
	s.notify("runs", houseFrom(r).ID, func(lang string) Payload {
		return Payload{
			Title: "🚗 " + houseFrom(r).Name + tr(lang, " gre v ", " drives to ") + str(m, "destination"),
			Body: join(" — ", tr(lang, "odhod ", "leaves ")+humanWhen(str(m, "cutoff_at"), lang, s.now()),
				tr(lang, "napiši, kaj rabiš", "post what you need")),
			URL: "#/market",
		}
	})
	writeJSON(w, 201, map[string]any{"id": id})
}

func (s *Server) listNeeds(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT x.*,`+houseJoin+`, t.name AS taken_by_name FROM needs x JOIN houses h ON h.id=x.house_id
		LEFT JOIN houses t ON t.id=x.taken_by WHERE x.state!='done' OR x.created_at >= datetime('now','-7 days') ORDER BY x.created_at DESC LIMIT 500`)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) createNeed(w http.ResponseWriter, r *http.Request) {
	m, err := readJSON(r)
	if err != nil || str(m, "text") == "" {
		writeErr(w, 400, "text required")
		return
	}
	var run any
	if v, ok := m["run_id"].(float64); ok {
		run = int64(v)
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO needs(house_id, text, run_id) VALUES (?,?,?)`, houseFrom(r).ID, str(m, "text"), run)
	if err != nil {
		fail(w, err)
		return
	}
	s.notify("needs", houseFrom(r).ID, func(lang string) Payload {
		return Payload{Title: "🛒 " + houseFrom(r).Name + tr(lang, " rabi iz trgovine", " needs from the shop"), Body: snippet(str(m, "text"), 140), URL: "#/market"}
	})
	writeJSON(w, 201, map[string]any{"id": id})
}

// updateNeed: anyone may take (state=taken, taken_by=me) or release; the owner
// or a steward may mark done or edit the text.
func (s *Server) updateNeed(w http.ResponseWriter, r *http.Request) {
	s.updateClaimable(w, r, "needs", "taken_by", "taken")
}

func (s *Server) listOffers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT x.*,`+houseJoin+`, t.name AS claimed_by_name FROM offers x JOIN houses h ON h.id=x.house_id
		LEFT JOIN houses t ON t.id=x.claimed_by WHERE x.state!='done' OR x.created_at >= datetime('now','-7 days') ORDER BY x.created_at DESC LIMIT 500`)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) createOffer(w http.ResponseWriter, r *http.Request) {
	m, err := readJSON(r)
	if err != nil || str(m, "text") == "" {
		writeErr(w, 400, "text required")
		return
	}
	tag := str(m, "tag")
	if tag != "seeds" && tag != "surplus" && tag != "joint" {
		tag = "giveaway"
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO offers(house_id, text, tag) VALUES (?,?,?)`, houseFrom(r).ID, str(m, "text"), tag)
	if err != nil {
		fail(w, err)
		return
	}
	s.notify("offers", houseFrom(r).ID, func(lang string) Payload {
		icon, what := "🎁", tr(lang, " podarja", " gives away")
		switch tag {
		case "seeds":
			icon, what = "🌱", tr(lang, " deli semena", " shares seeds")
		case "surplus":
			icon, what = "🧺", tr(lang, " deli presežek", " shares surplus")
		case "joint":
			icon, what = "📦", tr(lang, " zbira skupno naročilo", " starts a joint order")
		}
		return Payload{Title: icon + " " + houseFrom(r).Name + what, Body: snippet(str(m, "text"), 140), URL: "#/market"}
	})
	writeJSON(w, 201, map[string]any{"id": id})
}

func (s *Server) updateOffer(w http.ResponseWriter, r *http.Request) {
	s.updateClaimable(w, r, "offers", "claimed_by", "claimed")
}

// updateClaimable is the shared state machine for needs and offers:
// open -> claimedState (by any house) -> open (release by the claimer, owner or
// steward) or done (owner or steward). Text edits: owner or steward.
func (s *Server) updateClaimable(w http.ResponseWriter, r *http.Request, table, byCol, claimedState string) {
	h := houseFrom(r)
	id, _ := pathID(r)
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	row, err := s.st.One(r.Context(), `SELECT house_id, state, `+byCol+` AS by FROM `+table+` WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if row == nil {
		writeErr(w, 404, "not found")
		return
	}
	owner := row["house_id"].(int64) == h.ID || h.IsSteward
	claimer := row["by"] != nil && row["by"].(int64) == h.ID
	switch str(m, "state") {
	case claimedState:
		if row["state"] != "open" {
			writeErr(w, 409, "not open")
			return
		}
		s.st.Exec(r.Context(), `UPDATE `+table+` SET state=?, `+byCol+`=? WHERE id=?`, claimedState, h.ID, id)
	case "open":
		if !owner && !claimer {
			writeErr(w, 403, "not yours")
			return
		}
		s.st.Exec(r.Context(), `UPDATE `+table+` SET state='open', `+byCol+`=NULL WHERE id=?`, id)
	case "done":
		if !owner && !claimer {
			writeErr(w, 403, "not yours")
			return
		}
		s.st.Exec(r.Context(), `UPDATE `+table+` SET state='done' WHERE id=?`, id)
	}
	if t := str(m, "text"); t != "" {
		if !owner {
			writeErr(w, 403, "not yours")
			return
		}
		s.st.Exec(r.Context(), `UPDATE `+table+` SET text=? WHERE id=?`, t, id)
	}
	w.WriteHeader(204)
}

// ---- watchtower ----------------------------------------------------------

func (s *Server) listAway(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT x.*,`+houseJoin+`, t.name AS watcher_name FROM away x JOIN houses h ON h.id=x.house_id
		LEFT JOIN houses t ON t.id=x.watcher WHERE x.to_date >= date('now','-3 days') ORDER BY x.from_date`)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) createAway(w http.ResponseWriter, r *http.Request) {
	m, err := readJSON(r)
	if err != nil || str(m, "from_date") == "" || str(m, "to_date") == "" {
		writeErr(w, 400, "from_date and to_date required")
		return
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO away(house_id, from_date, to_date, notes) VALUES (?,?,?,?)`, houseFrom(r).ID, str(m, "from_date"), str(m, "to_date"), str(m, "notes"))
	if err != nil {
		fail(w, err)
		return
	}
	// Away notices are burglary information and a lock screen is readable by
	// anyone holding the phone: the push names no house, no dates, no notes.
	s.notify("away", houseFrom(r).ID, func(lang string) Payload {
		return Payload{Title: tr(lang, "🕯️ Stražnica", "🕯️ Watchtower"), Body: tr(lang, "nekdo je vpisal odsotnost — odpri portal", "someone marked an absence — open the portal"), URL: "#/watch"}
	})
	writeJSON(w, 201, map[string]any{"id": id})
}

// updateAway: any house may become the watcher (or step back); the owner or a
// steward edits dates and notes.
func (s *Server) updateAway(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, _ := pathID(r)
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	if v, ok := m["watch"].(bool); ok {
		if v {
			s.st.Exec(r.Context(), `UPDATE away SET watcher=? WHERE id=? AND watcher IS NULL`, h.ID, id)
		} else {
			s.st.Exec(r.Context(), `UPDATE away SET watcher=NULL WHERE id=? AND (watcher=? OR ?)`, id, h.ID, h.IsSteward)
		}
	}
	edits := false
	for _, k := range []string{"from_date", "to_date", "notes"} {
		if _, ok := m[k]; ok {
			edits = true
		}
	}
	if edits {
		if !s.ownerOrSteward(w, r, "away", id) {
			return
		}
		for _, k := range []string{"from_date", "to_date", "notes"} {
			if _, ok := m[k]; ok {
				s.st.Exec(r.Context(), `UPDATE away SET `+k+`=? WHERE id=?`, str(m, k), id)
			}
		}
	}
	w.WriteHeader(204)
}

// ---- export --------------------------------------------------------------

func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	out := map[string]any{"exported_at": time.Now().UTC().Format(time.RFC3339)}
	for _, t := range []string{"houses", "house_parcels", "posts", "events", "event_signups", "runs", "needs", "offers", "away", "tools", "wishes", "wish_wants", "projects", "project_tasks", "camp_takings"} {
		cols := "*"
		if t == "tools" { // photos are bytes, not text — they stay in the SQLite backup
			cols = "id, house_id, name, notes, category, held_by, held_since, reminded_at, created_at"
		}
		rows, err := s.st.Rows(r.Context(), `SELECT `+cols+` FROM `+t)
		if err != nil {
			fail(w, err)
			return
		}
		out[t] = rows
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="potok-export-%s.json"`, time.Now().UTC().Format("2006-01-02")))
	writeJSON(w, 200, out)
}

// ---- work-bee sign-ups ---------------------------------------------------

// signUp puts this house on an event's list and tells the house that called
// the work bee. No count is ever shown as a score — it is a headcount for the
// day, deleted with the event.
func (s *Server) signUp(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "bad id")
		return
	}
	m, _ := readJSON(r)
	ev, err := s.st.One(r.Context(), `SELECT house_id, title FROM events WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if ev == nil {
		writeErr(w, 404, "no such event")
		return
	}
	if _, err := s.st.Exec(r.Context(), `INSERT INTO event_signups(event_id, house_id, note) VALUES (?,?,?)
		ON CONFLICT(event_id, house_id) DO UPDATE SET note=excluded.note`, id, h.ID, str(m, "note")); err != nil {
		fail(w, err)
		return
	}
	if owner := ev["house_id"].(int64); owner != h.ID {
		title := ev["title"].(string)
		s.notifyHouse("events", owner, func(lang string) Payload {
			return Payload{Title: "🙋 " + h.Name + tr(lang, " prihaja", " is coming"), Body: title, URL: "#/bell"}
		})
	}
	w.WriteHeader(204)
}

func (s *Server) signOff(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "bad id")
		return
	}
	if _, err := s.st.Exec(r.Context(), `DELETE FROM event_signups WHERE event_id=? AND house_id=?`, id, houseFrom(r).ID); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(204)
}

// ---- tool shed -----------------------------------------------------------

func (s *Server) listTools(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), `SELECT x.id, x.house_id, x.name, x.notes, x.category, x.held_by, x.held_since, x.created_at,
		(x.photo IS NOT NULL) AS has_photo,`+houseJoin+`, t.name AS held_by_name, t.crest AS held_by_crest
		FROM tools x JOIN houses h ON h.id=x.house_id LEFT JOIN houses t ON t.id=x.held_by ORDER BY h.name, x.name`)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) createTool(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	m, err := readJSON(r)
	if err != nil || str(m, "name") == "" {
		writeErr(w, 400, "name required")
		return
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO tools(house_id, name, notes, category) VALUES (?,?,?,?)`, h.ID, str(m, "name"), str(m, "notes"), validCategory(str(m, "category")))
	if err != nil {
		fail(w, err)
		return
	}
	s.notify("tools", h.ID, func(lang string) Payload {
		return Payload{Title: "🛠 " + h.Name + tr(lang, " deli orodje", " shares a tool"), Body: snippet(join(" — ", str(m, "name"), str(m, "notes")), 140), URL: "#/shed"}
	})
	writeJSON(w, 201, map[string]any{"id": id})
}

// updateTool: any house takes a free tool or gives it back; the owner or a
// steward edits the name and notes. Taking tells the owner, and nothing else.
func (s *Server) updateTool(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "bad id")
		return
	}
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	row, err := s.st.One(r.Context(), `SELECT house_id, name, held_by FROM tools WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if row == nil {
		writeErr(w, 404, "not found")
		return
	}
	owner := row["house_id"].(int64)
	mayEdit := owner == h.ID || h.IsSteward
	holder, held := row["held_by"].(int64)

	if take, ok := m["take"].(bool); ok {
		if take {
			if held {
				writeErr(w, 409, "already taken")
				return
			}
			s.st.Exec(r.Context(), `UPDATE tools SET held_by=?, held_since=date('now') WHERE id=?`, h.ID, id)
			if owner != h.ID {
				name := row["name"].(string)
				s.notifyHouse("tools", owner, func(lang string) Payload {
					return Payload{Title: "🛠 " + h.Name + tr(lang, " je vzel", " took"), Body: name, URL: "#/shed"}
				})
			}
		} else {
			if held && holder != h.ID && !mayEdit {
				writeErr(w, 403, "not yours to return")
				return
			}
			s.st.Exec(r.Context(), `UPDATE tools SET held_by=NULL, held_since=NULL, reminded_at=NULL WHERE id=?`, id)
			if held && owner != h.ID {
				name := row["name"].(string)
				s.notifyHouse("tools", owner, func(lang string) Payload {
					return Payload{Title: "🛠 " + h.Name + tr(lang, " je vrnil", " returned"), Body: name, URL: "#/shed"}
				})
			}
		}
	}
	for _, k := range []string{"name", "notes", "category"} {
		if _, ok := m[k]; ok {
			if !mayEdit {
				writeErr(w, 403, "not yours")
				return
			}
			v := str(m, k)
			if k == "category" {
				v = validCategory(v)
			}
			s.st.Exec(r.Context(), `UPDATE tools SET `+k+`=? WHERE id=?`, v, id)
		}
	}
	w.WriteHeader(204)
}

// ---- pairing -------------------------------------------------------------

// createPairing hands the caller a six-digit code so one more phone of the
// same house can log in. Only one live code per house.
func (s *Server) createPairing(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	if _, err := s.st.Exec(r.Context(), `DELETE FROM pairings WHERE house_id=? OR expires_at < datetime('now')`, h.ID); err != nil {
		fail(w, err)
		return
	}
	code := pairingCode()
	exp := time.Now().UTC().Add(15 * time.Minute).Format(time.RFC3339)
	if _, err := s.st.Exec(r.Context(), `INSERT INTO pairings(code, house_id, expires_at) VALUES (?,?,?)`, code, h.ID, exp); err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"code": code, "expires_at": exp})
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
