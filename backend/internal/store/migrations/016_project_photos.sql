-- Pictures on a project: the fence half-built, the roof before and after.
-- Any house may add one; shrunk in the browser, kept as a blob like a tool
-- photo, so the nightly SQLite backup still carries the whole village.
CREATE TABLE project_photos (
  id          INTEGER PRIMARY KEY,
  project_id  INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  photo       BLOB NOT NULL,
  photo_type  TEXT NOT NULL,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX project_photos_project ON project_photos(project_id);
