-- The codex's words move out of per-language columns into one row per
-- language, so a village's third language is a row and not a migration.
-- ON DELETE CASCADE: a section's texts go with it. The four columns are
-- copied over and dropped; a section keeps its ord, rev, stamp and house.
CREATE TABLE codex_texts (
  section_id INTEGER NOT NULL REFERENCES codex_sections(id) ON DELETE CASCADE,
  lang       TEXT NOT NULL,
  title      TEXT NOT NULL DEFAULT '',
  body       TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (section_id, lang)
);
INSERT INTO codex_texts (section_id, lang, title, body)
  SELECT id, 'sl', title_sl, body_sl FROM codex_sections WHERE title_sl <> '' OR body_sl <> '';
INSERT INTO codex_texts (section_id, lang, title, body)
  SELECT id, 'en', title_en, body_en FROM codex_sections WHERE title_en <> '' OR body_en <> '';
ALTER TABLE codex_sections DROP COLUMN title_sl;
ALTER TABLE codex_sections DROP COLUMN title_en;
ALTER TABLE codex_sections DROP COLUMN body_sl;
ALTER TABLE codex_sections DROP COLUMN body_en;
