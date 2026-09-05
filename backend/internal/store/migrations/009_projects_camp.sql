-- Projects: a long job, its tasks, and events that belong to it.
CREATE TABLE projects (
  id          INTEGER PRIMARY KEY,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  title       TEXT NOT NULL,
  notes       TEXT NOT NULL DEFAULT '',
  due_at      TEXT,
  state       TEXT NOT NULL DEFAULT 'open',
  done_at     TEXT,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- A task is taken, never assigned: assigned_to is only ever set by the house
-- that takes it (owner's decision 2026-09-05).
CREATE TABLE project_tasks (
  id            INTEGER PRIMARY KEY,
  project_id    INTEGER NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  house_id      INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  title         TEXT NOT NULL,
  notes         TEXT NOT NULL DEFAULT '',
  assigned_to   INTEGER REFERENCES houses(id) ON DELETE SET NULL,
  due_at        TEXT,
  state         TEXT NOT NULL DEFAULT 'open',
  done_at       TEXT,
  closing_note  TEXT NOT NULL DEFAULT '',
  created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

ALTER TABLE events ADD COLUMN project_id INTEGER REFERENCES projects(id) ON DELETE SET NULL;
ALTER TABLE events ADD COLUMN task_id INTEGER REFERENCES project_tasks(id) ON DELETE SET NULL;

-- Campground: one row = one house collected from one camper. No amounts, by
-- the owner's decision 2026-09-05: the portal says who collected and a note,
-- the cash box says how much.
CREATE TABLE camp_takings (
  id            INTEGER PRIMARY KEY,
  house_id      INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  collected_by  TEXT NOT NULL DEFAULT '',
  from_who      TEXT NOT NULL,
  taken_on      TEXT NOT NULL,
  notes         TEXT NOT NULL DEFAULT '',
  state         TEXT NOT NULL DEFAULT 'held',
  handed_at     TEXT,
  created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);
