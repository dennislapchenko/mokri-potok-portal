-- A picture outlives the house that added it (owner's decision 2026-09-08):
-- the fence is still the village's fence after the neighbour who photographed
-- it has left. house_id goes NULL instead of taking the row with it. SQLite
-- cannot change a foreign key in place, so the table is rebuilt.
CREATE TABLE project_photos_new (
  id          INTEGER PRIMARY KEY,
  project_id  INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  house_id    INTEGER REFERENCES houses(id) ON DELETE SET NULL,
  photo       BLOB NOT NULL,
  photo_type  TEXT NOT NULL,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO project_photos_new (id, project_id, house_id, photo, photo_type, created_at)
  SELECT id, project_id, house_id, photo, photo_type, created_at FROM project_photos;
DROP TABLE project_photos;
ALTER TABLE project_photos_new RENAME TO project_photos;
CREATE INDEX project_photos_project ON project_photos(project_id);
