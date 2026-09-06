-- Sign-ups become an answer, not a fact: yes, no or maybe. A confirmed "not
-- coming" is worth more than silence, so it is a first-class state.
ALTER TABLE event_signups ADD COLUMN state TEXT NOT NULL DEFAULT 'yes';

-- Comments on an event, one reply level, same shape as the tavern board. They
-- replace the single note a sign-up used to carry; old notes stay readable.
CREATE TABLE event_comments (
  id          INTEGER PRIMARY KEY,
  event_id    INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  parent_id   INTEGER REFERENCES event_comments(id) ON DELETE CASCADE,
  author      TEXT NOT NULL DEFAULT '',
  body        TEXT NOT NULL,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_event_comments_event ON event_comments(event_id);

-- Any house may edit an event now, so the room must say who last touched it,
-- and an answer given before the time moved must not read as current.
ALTER TABLE events ADD COLUMN edited_by INTEGER REFERENCES houses(id) ON DELETE SET NULL;
ALTER TABLE events ADD COLUMN edited_at TEXT;
ALTER TABLE events ADD COLUMN time_changed_at TEXT;
ALTER TABLE event_signups ADD COLUMN answered_at TEXT;
UPDATE event_signups SET answered_at = created_at;

-- Staleness is a counter, not a clock: an answer and a time change inside the
-- same second are indistinguishable by timestamp, and SQLite's datetime('now')
-- only has second resolution.
ALTER TABLE events ADD COLUMN time_version INTEGER NOT NULL DEFAULT 0;
ALTER TABLE event_signups ADD COLUMN answered_version INTEGER NOT NULL DEFAULT 0;
