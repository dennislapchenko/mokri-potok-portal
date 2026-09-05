-- Tool shed grows up: a category, a photo (resized in the browser, kept as a
-- blob so the backup carries it), and a wishlist of tools the village lacks.
ALTER TABLE tools ADD COLUMN category TEXT NOT NULL DEFAULT 'other';
ALTER TABLE tools ADD COLUMN photo BLOB;
ALTER TABLE tools ADD COLUMN photo_type TEXT;

CREATE TABLE wishes (
  id          INTEGER PRIMARY KEY,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  text        TEXT NOT NULL,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Houses that would also love the village to have this. Names, not a score.
CREATE TABLE wish_wants (
  wish_id   INTEGER NOT NULL REFERENCES wishes(id) ON DELETE CASCADE,
  house_id  INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  PRIMARY KEY (wish_id, house_id)
);
