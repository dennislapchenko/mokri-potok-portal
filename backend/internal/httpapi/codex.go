package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
)

// The Codex is the village's own text: the values and agreements the council
// adopted, kept as ordered sections with one text per language of the village
// (codex_texts, keyed by LANGUAGES). Any house may edit a section or add one —
// every account today is a villager, the same footing as editing an event —
// and the section says which house last wrote it and when. A steward removes
// a section. Nothing here pushes: a changed codex is announced in the Tavern
// by the house that changed it, in its own words.

const codexSelect = `SELECT c.*, h.name AS house_name, h.crest AS house_crest, h.color AS house_color
	FROM codex_sections c LEFT JOIN houses h ON h.id=c.updated_by ORDER BY c.ord, c.id`

type codexText struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

// texts reads {"texts": {"sl": {"title": …, "body": …}}} from a body. A
// language the village does not speak is refused, not stored: the page only
// ever offers the village's own.
func (s *Server) codexTexts(m map[string]any) (map[string]codexText, error) {
	raw, _ := m["texts"].(map[string]any)
	out := map[string]codexText{}
	for lang, v := range raw {
		if !slices.Contains(s.cfg.Languages, lang) {
			return nil, fmt.Errorf("the village does not speak %q", lang)
		}
		tm, _ := v.(map[string]any)
		out[lang] = codexText{Title: str(tm, "title"), Body: str(tm, "body")}
	}
	return out, nil
}

func hasTitle(texts map[string]codexText) bool {
	for _, t := range texts {
		if t.Title != "" {
			return true
		}
	}
	return false
}

// writeTexts upserts the given languages on a section; a text that is empty
// in both fields is removed, so "borrowed" on the page means what it says.
func (s *Server) writeTexts(ctx context.Context, id int64, texts map[string]codexText) error {
	for lang, t := range texts {
		var err error
		if t.Title == "" && t.Body == "" {
			_, err = s.st.Exec(ctx, `DELETE FROM codex_texts WHERE section_id=? AND lang=?`, id, lang)
		} else {
			_, err = s.st.Exec(ctx, `INSERT INTO codex_texts (section_id, lang, title, body) VALUES (?,?,?,?)
				ON CONFLICT(section_id, lang) DO UPDATE SET title=excluded.title, body=excluded.body`, id, lang, t.Title, t.Body)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) listCodex(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Rows(r.Context(), codexSelect)
	if err != nil {
		fail(w, err)
		return
	}
	texts, err := s.st.Rows(r.Context(), `SELECT section_id, lang, title, body FROM codex_texts`)
	if err != nil {
		fail(w, err)
		return
	}
	by := map[int64]map[string]codexText{}
	for _, t := range texts {
		id := t["section_id"].(int64)
		if by[id] == nil {
			by[id] = map[string]codexText{}
		}
		by[id][t["lang"].(string)] = codexText{Title: t["title"].(string), Body: t["body"].(string)}
	}
	for _, row := range rows {
		m := by[row["id"].(int64)]
		if m == nil {
			m = map[string]codexText{}
		}
		row["texts"] = m
	}
	writeJSON(w, 200, rows)
}

func (s *Server) createCodexSection(w http.ResponseWriter, r *http.Request) {
	m, err := readJSON(r)
	if err != nil {
		writeErr(w, 400, "bad json")
		return
	}
	texts, err := s.codexTexts(m)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	if !hasTitle(texts) {
		writeErr(w, 400, "a section needs a title")
		return
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO codex_sections(ord, updated_by)
		VALUES ((SELECT COALESCE(MAX(ord),0)+1 FROM codex_sections), ?)`, houseFrom(r).ID)
	if err != nil {
		fail(w, err)
		return
	}
	if err := s.writeTexts(r.Context(), id, texts); err != nil {
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
	texts, err := s.codexTexts(m)
	if err != nil {
		writeErr(w, 400, err.Error())
		return
	}
	// One statement, compare-and-set on rev, claims the section before any
	// text is written. A form sends the rev it opened on; if the section moved
	// since, two houses were writing the same sentence and the second one is
	// told, so neither overwrites the other unknowingly. A request without a
	// rev (a script) is taken as it is. Only the languages sent are touched.
	args := []any{houseFrom(r).ID, id}
	where := ""
	if rev, ok := m["rev"].(float64); ok {
		where = " AND rev=?"
		args = append(args, int64(rev))
	}
	n, err := s.st.ExecN(r.Context(), `UPDATE codex_sections SET rev=rev+1, updated_at=datetime('now'), updated_by=? WHERE id=?`+where, args...)
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
	if err := s.writeTexts(r.Context(), id, texts); err != nil {
		fail(w, err)
		return
	}
	w.WriteHeader(204)
}

// ---- import --------------------------------------------------------------

// The adopted text enters once, from a JSON file on the VM, so the repo never
// carries it:
//
//	docker exec -i <container> /server codex-import < codex.json
//
// {"adopted": "2025-01-26", "sections": [{"texts": {"sl": {"title": …, "body": …}, "en": {…}}}]}
//
// Every section is stamped with the adoption date and no house, which the page
// reads as "adopted by the village council". It refuses to run into a codex
// that already has sections: a second import would print the text twice, and
// once the houses have edited, the file is the older copy.

var isoDate = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type codexSeed struct {
	Adopted  string `json:"adopted"`
	Sections []struct {
		Texts map[string]codexText `json:"texts"`
	} `json:"sections"`
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
	for i, sec := range seed.Sections {
		if !hasTitle(sec.Texts) {
			return fmt.Errorf("section %d has no title", i+1)
		}
		for lang := range sec.Texts {
			if !slices.Contains(s.cfg.Languages, lang) {
				return fmt.Errorf("section %d: the village does not speak %q (LANGUAGES)", i+1, lang)
			}
		}
	}
	// Noon, so the date reads the same in every timezone the page is opened in.
	at := seed.Adopted + " 12:00:00"
	for i, sec := range seed.Sections {
		id, err := s.st.Exec(ctx, `INSERT INTO codex_sections(ord, updated_at, updated_by) VALUES (?,?,NULL)`, i+1, at)
		if err != nil {
			return err
		}
		if err := s.writeTexts(ctx, id, sec.Texts); err != nil {
			return err
		}
	}
	fmt.Printf("codex: %d sections, adopted %s\n", len(seed.Sections), seed.Adopted)
	return nil
}

// deleteCodexSection is steward-only (the route says so): a section vanishing
// is a bigger act than a sentence changing, and the nightly backup is its undo.
// Its texts go with it (ON DELETE CASCADE).
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
