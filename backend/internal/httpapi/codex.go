package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

// The Codex is the village's own text: the values and agreements the council
// adopted, kept as ordered bilingual sections. Any house may edit a section or
// add one — every account today is a villager, the same footing as editing an
// event — and the section says which house last wrote it and when. A steward
// removes a section. Nothing here pushes: a changed codex is announced in the
// Tavern by the house that changed it, in its own words.

var codexFields = []string{"title_sl", "title_en", "body_sl", "body_en"}

const codexSelect = `SELECT c.*, h.name AS house_name, h.crest AS house_crest, h.color AS house_color
	FROM codex_sections c LEFT JOIN houses h ON h.id=c.updated_by ORDER BY c.ord, c.id`

func (s *Server) listCodex(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), codexSelect)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

func (s *Server) createCodexSection(w http.ResponseWriter, r *http.Request) {
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	if str(m, "title_sl") == "" && str(m, "title_en") == "" {
		writeErr(w, 400, "a section needs a title")
		return
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO codex_sections(ord, title_sl, title_en, body_sl, body_en, updated_by)
		VALUES ((SELECT COALESCE(MAX(ord),0)+1 FROM codex_sections), ?,?,?,?, ?)`,
		str(m, "title_sl"), str(m, "title_en"), str(m, "body_sl"), str(m, "body_en"), houseFrom(r).ID)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id})
}

func (s *Server) updateCodexSection(w http.ResponseWriter, r *http.Request) {
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
	// One statement, compare-and-set on rev. A form sends the rev it opened
	// on; if the section moved since, two houses were writing the same
	// sentence and the second one is told, so neither overwrites the other
	// unknowingly. A request without a rev (a script) is taken as it is.
	set, args := "", []any{}
	for _, k := range codexFields {
		if _, ok := m[k]; ok {
			set += k + "=?, "
			args = append(args, str(m, k))
		}
	}
	args = append(args, houseFrom(r).ID, id)
	where := ""
	if rev, ok := m["rev"].(float64); ok {
		where = " AND rev=?"
		args = append(args, int64(rev))
	}
	n, err := s.st.ExecN(r.Context(), `UPDATE codex_sections SET `+set+`rev=rev+1, updated_at=datetime('now'), updated_by=? WHERE id=?`+where, args...)
	if err != nil {
		fail(w, err)
		return
	}
	if n == 0 {
		if row, _ := s.st.One(r.Context(), `SELECT id FROM codex_sections WHERE id=?`, id); row == nil {
			writeErr(w, 404, "not found")
		} else {
			writeErr(w, 409, "changed meanwhile")
		}
		return
	}
	w.WriteHeader(204)
}

// ---- import --------------------------------------------------------------

// The adopted text enters once, from a JSON file on the VM, so the repo never
// carries it:
//
//	docker exec -i mokri-potok-potok-api-1 /server codex-import < codex.json
//
// {"adopted": "2025-01-26", "sections": [{"title_sl": …, "body_sl": …, "title_en": …, "body_en": …}]}
//
// Every section is stamped with the adoption date and no house, which the page
// reads as "adopted by the village council". It refuses to run into a codex
// that already has sections: a second import would print the text twice, and
// once the houses have edited, the file is the older copy.

var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type codexSeed struct {
	Adopted  string              `json:"adopted"`
	Sections []map[string]string `json:"sections"`
}

func (s *Server) ImportCodex(in io.Reader) error {
	ctx := context.Background()
	var seed codexSeed
	if err := json.NewDecoder(in).Decode(&seed); err != nil {
		return fmt.Errorf("read seed: %w", err)
	}
	if !isoDate.MatchString(seed.Adopted) {
		return fmt.Errorf("adopted must be YYYY-MM-DD, got %q", seed.Adopted)
	}
	if len(seed.Sections) == 0 {
		return fmt.Errorf("no sections in the seed")
	}
	row, err := s.st.One(ctx, `SELECT count(*) AS n FROM codex_sections`)
	if err != nil {
		return err
	}
	if row["n"].(int64) > 0 {
		return fmt.Errorf("the codex already has %d sections; edit it in the app instead", row["n"])
	}
	// Noon, so the date reads the same in every timezone the page is opened in.
	at := seed.Adopted + " 12:00:00"
	for i, sec := range seed.Sections {
		if sec["title_sl"] == "" && sec["title_en"] == "" {
			return fmt.Errorf("section %d has no title", i+1)
		}
		if _, err := s.st.Exec(ctx, `INSERT INTO codex_sections(ord, title_sl, title_en, body_sl, body_en, updated_at, updated_by)
			VALUES (?,?,?,?,?,?,NULL)`, i+1, sec["title_sl"], sec["title_en"], sec["body_sl"], sec["body_en"], at); err != nil {
			return err
		}
	}
	fmt.Printf("codex: %d sections, adopted %s\n", len(seed.Sections), seed.Adopted)
	return nil
}

// deleteCodexSection is steward-only (the route says so): a section vanishing
// is a bigger act than a sentence changing, and the nightly backup is its undo.
func (s *Server) deleteCodexSection(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "bad id")
		return
	}
	row, err := s.st.One(r.Context(), `SELECT id FROM codex_sections WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if row == nil {
		writeErr(w, 404, "not found")
		return
	}
	if _, err := s.st.Exec(r.Context(), `DELETE FROM codex_sections WHERE id=?`, id); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(204)
}
