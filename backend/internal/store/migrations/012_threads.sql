-- Comments now hang on more than events: a wish in the tool shed gets them too,
-- and the next thing will as well. One table, keyed by what it is attached to,
-- instead of a table per room.
CREATE TABLE comments (
  id          INTEGER PRIMARY KEY,
  subject     TEXT NOT NULL,            -- 'event' | 'wish'
  subject_id  INTEGER NOT NULL,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  parent_id   INTEGER REFERENCES comments(id) ON DELETE CASCADE,
  author      TEXT NOT NULL DEFAULT '',
  body        TEXT NOT NULL,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_comments_subject ON comments(subject, subject_id);

-- Carry the event comments over, ids and reply links intact.
INSERT INTO comments (id, subject, subject_id, house_id, parent_id, author, body, created_at)
SELECT id, 'event', event_id, house_id, parent_id, author, body, created_at FROM event_comments;
DROP TABLE event_comments;

-- A wish can collect options: "this one, 180 EUR, link". Anyone may add one.
CREATE TABLE wish_options (
  id          INTEGER PRIMARY KEY,
  wish_id     INTEGER NOT NULL REFERENCES wishes(id) ON DELETE CASCADE,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  text        TEXT NOT NULL,
  url         TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_wish_options_wish ON wish_options(wish_id);
