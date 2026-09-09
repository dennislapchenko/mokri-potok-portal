-- The Codex: the village's founding text — its values and the agreements the
-- village council adopted — as ordered, bilingual sections. It lives in the
-- database and not in the repo (owner's decision 2026-09-09): the text is the
-- village's to change, the repo is public, and a constitution that needs a
-- deploy to amend belongs to the builder, not to the houses.
--
-- updated_by is the house that last wrote the section; NULL means the text
-- stands as the council adopted it. ON DELETE SET NULL: a section outlives the
-- house that edited it, like a picture on a project. rev counts the writes:
-- a form sends the rev it opened on and an update is refused when it moved,
-- so two houses on one sentence never overwrite each other unknowingly. A
-- counter, not the stamp — a stamp has seconds, and two saves in one second
-- would read as one.
CREATE TABLE codex_sections (
  id         INTEGER PRIMARY KEY,
  ord        INTEGER NOT NULL,
  title_sl   TEXT NOT NULL DEFAULT '',
  title_en   TEXT NOT NULL DEFAULT '',
  body_sl    TEXT NOT NULL DEFAULT '',
  body_en    TEXT NOT NULL DEFAULT '',
  rev        INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  updated_by INTEGER REFERENCES houses(id) ON DELETE SET NULL
);
