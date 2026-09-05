-- Who muted the village, and when. Silent power is the problem, not the lever.
ALTER TABLE notify_off_global ADD COLUMN set_by INTEGER REFERENCES houses(id) ON DELETE SET NULL;
ALTER TABLE notify_off_global ADD COLUMN set_at TEXT NOT NULL DEFAULT (datetime('now'));
