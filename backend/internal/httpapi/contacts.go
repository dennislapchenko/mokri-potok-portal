package httpapi

import (
	"net/http"
	"strings"
)

// Contacts: the village's phone book — the well-driller, the vet, the man with
// the plough. Any house adds a number and any house corrects it, the same
// footing as editing an event, and the row names the house that last wrote it.
// A contact outlives the house that added it, so both house columns may be
// NULL and every join to houses is a LEFT one.
//
// Nothing here pushes (owner's decision 2026-09-10): a phone book is a thing
// you look up when the pipe bursts, not news, and a tenth kind in every
// house's notification list buys nobody anything. The cost, paid knowingly: a
// comment saying "this number is dead" waits until somebody opens the room.

var contactFields = []string{"name", "phone", "notes", "type"}

func (s *Server) listContacts(w http.ResponseWriter, r *http.Request) {
	// Untyped last: a row with no type wears no pill saying what it is, so it
	// is the one the eye should meet after the named ones, not before. Then by
	// type, so like sits with like, then by name inside it. A type beginning
	// with Č or Š sorts after Z, which moves that block of rows and splits
	// nothing.
	//
	// The house that wrote a number down is an id and no more: the card does
	// not name it, so neither the page nor an agent is handed its name, crest
	// and colour. Only the house that last changed the row is named.
	rows, err := s.st.Rows(r.Context(), `SELECT c.*, ed.name AS edited_by_name,
		(SELECT count(*) FROM comments m WHERE m.subject='contact' AND m.subject_id=c.id) AS comments
		FROM contacts c LEFT JOIN houses ed ON ed.id=c.edited_by
		ORDER BY c.type='', c.type COLLATE NOCASE, c.name COLLATE NOCASE`)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 200, rows)
}

// contactType settles the spelling of a type. Two houses writing "Vet" and
// "vet" would give the picker two rows for one thing, and nobody would know
// which to pick — so a type that already exists is kept in the spelling it
// already has, and only a genuinely new word enters as it was typed.
//
// The comparison is Go's, not SQLite's: COLLATE NOCASE folds A–Z and nothing
// else, so "Čebelar" and "čebelar" would pass it as two types in a Slovenian
// village. strings.EqualFold folds the whole alphabet, and it is the same
// answer the picker's toLowerCase() gives, so the two never disagree.
//
// `self` is the row being written, excluded from the comparison: a house
// correcting the only "obrtnik" row to "Obrtnik" is renaming the type, not
// colliding with it, and a save that quietly hands back the old spelling is
// the silent clamp the date picker refuses.
func (s *Server) contactType(r *http.Request, typ string, self int64) string {
	typ = strings.Join(strings.Fields(typ), " ")
	if typ == "" {
		return ""
	}
	rows, _ := s.st.Rows(r.Context(), `SELECT DISTINCT type FROM contacts WHERE type<>'' AND id<>?`, self)
	for _, row := range rows {
		if in := str(row, "type"); strings.EqualFold(in, typ) {
			return in
		}
	}
	return typ
}

func (s *Server) createContact(w http.ResponseWriter, r *http.Request) {
	m, err := readJSON(r)
	if err != nil || str(m, "name") == "" {
		writeErr(w, 400, "name required")
		return
	}
	id, err := s.st.Exec(r.Context(), `INSERT INTO contacts(house_id, name, phone, notes, type) VALUES (?,?,?,?,?)`,
		houseFrom(r).ID, str(m, "name"), str(m, "phone"), str(m, "notes"), s.contactType(r, str(m, "type"), 0))
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, 201, map[string]any{"id": id})
}

// updateContact: any house may correct any contact — a wrong number helps
// nobody, and every account today is a villager. The row then carries that
// house's name, so a text anyone may change shows who changed it.
func (s *Server) updateContact(w http.ResponseWriter, r *http.Request) {
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
	if v, ok := m["name"]; ok {
		if name, _ := v.(string); name == "" {
			writeErr(w, 400, "a contact needs a name")
			return
		}
	}
	row, err := s.st.One(r.Context(), `SELECT id FROM contacts WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if row == nil {
		writeErr(w, 404, "not found")
		return
	}
	for _, k := range contactFields {
		if _, ok := m[k]; !ok {
			continue
		}
		v := str(m, k)
		if k == "type" {
			v = s.contactType(r, v, id)
		}
		if _, err := s.st.Exec(r.Context(), `UPDATE contacts SET `+k+`=? WHERE id=?`, v, id); err != nil {
			fail(w, err)
			return
		}
	}
	s.st.Exec(r.Context(), `UPDATE contacts SET edited_by=?, edited_at=datetime('now') WHERE id=?`, houseFrom(r).ID, id)
	w.WriteHeader(204)
}

// deleteContact: the house that added it, or a steward. Correcting a number is
// everyone's, removing one is not — and once the adding house is gone the row
// belongs to the village, so only a steward takes it away.
func (s *Server) deleteContact(w http.ResponseWriter, r *http.Request) {
	h := houseFrom(r)
	id, err := pathID(r)
	if err != nil {
		writeErr(w, 400, "bad id")
		return
	}
	row, err := s.st.One(r.Context(), `SELECT house_id FROM contacts WHERE id=?`, id)
	if err != nil {
		fail(w, err)
		return
	}
	if row == nil {
		writeErr(w, 404, "not found")
		return
	}
	if owner, ok := row["house_id"].(int64); !h.IsSteward && (!ok || owner != h.ID) {
		writeErr(w, 403, "not yours")
		return
	}
	if _, err := s.st.Exec(r.Context(), `DELETE FROM contacts WHERE id=?`, id); err != nil {
		fail(w, err)
		return
	}
	// The thread goes with the number. Ids are reused in SQLite once the
	// highest row is gone, so a comment left behind would surface under
	// whatever contact is written next.
	s.st.Exec(r.Context(), `DELETE FROM comments WHERE subject='contact' AND subject_id=?`, id)
	w.WriteHeader(204)
}
