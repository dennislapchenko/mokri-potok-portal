-- Pictures the page wears that a village may replace: today the backdrop
-- behind the gate. In the database like the photos and the map, so the backup
-- is the whole village; absent means the binary's own default serves. One row
-- per name; the bytes keep their content type.
CREATE TABLE site_files (
  name         TEXT PRIMARY KEY,
  bytes        BLOB NOT NULL,
  content_type TEXT NOT NULL,
  updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
