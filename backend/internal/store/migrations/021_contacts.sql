-- Contacts: the numbers the village asks for again every winter — the
-- well-driller, the vet, the chimney sweep, the man with the plough. The chat
-- has them all, once, somewhere above.
--
-- Any house adds one and any house corrects it, the same provisional footing
-- as editing an event, so the row records which house last wrote it and when.
--
-- A contact outlives the house that added it, like a project picture and a
-- codex section: the plumber is still the village's plumber after the house
-- that found him has gone. Both house columns go NULL instead of taking the
-- row with them.
--
-- `type` is one word a house typed — vet, craftsman, office. There is no table
-- of types on purpose: the picker offers the types already in use, so a new
-- one exists the moment somebody writes it, and the last row carrying a type
-- takes it away again. A list of two houses' spellings is worse than no list,
-- so the backend reuses an existing spelling that differs only in case.
CREATE TABLE contacts (
  id          INTEGER PRIMARY KEY,
  house_id    INTEGER REFERENCES houses(id) ON DELETE SET NULL,
  name        TEXT NOT NULL,
  phone       TEXT NOT NULL DEFAULT '',
  notes       TEXT NOT NULL DEFAULT '',
  type        TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (datetime('now')),
  edited_at   TEXT,
  edited_by   INTEGER REFERENCES houses(id) ON DELETE SET NULL
);
CREATE INDEX idx_contacts_type ON contacts(type, name);
