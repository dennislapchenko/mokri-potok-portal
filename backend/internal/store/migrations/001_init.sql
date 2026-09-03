-- Houses are the accounts. kind: house | common (the collective's shared land,
-- e.g. the event grounds). is_steward houses can create houses and pin posts.
CREATE TABLE houses (
  id          INTEGER PRIMARY KEY,
  name        TEXT NOT NULL,
  crest       TEXT NOT NULL DEFAULT '🏠',
  color       TEXT NOT NULL DEFAULT '#b5651d',
  kind        TEXT NOT NULL DEFAULT 'house',
  is_steward  INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- One row per parcel a house holds. Parcel numbers come from the public
-- cadastre file shipped with the frontend; assignment is app data, never in git.
CREATE TABLE house_parcels (
  house_id  INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  parcel    TEXT NOT NULL,
  PRIMARY KEY (parcel)
);

-- Invite codes: a house has at most one live code. Opening the link once on a
-- phone creates a device below; the code itself stays valid until expires_at
-- so every member of the house can use the same link.
CREATE TABLE invites (
  code        TEXT PRIMARY KEY,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  expires_at  TEXT NOT NULL,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- A device is one logged-in browser. The token is stored hashed.
CREATE TABLE devices (
  id          INTEGER PRIMARY KEY,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  token_hash  TEXT NOT NULL UNIQUE,
  label       TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (datetime('now')),
  last_seen   TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Tavern: board posts and replies.
CREATE TABLE posts (
  id          INTEGER PRIMARY KEY,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  parent_id   INTEGER REFERENCES posts(id) ON DELETE CASCADE,
  author      TEXT NOT NULL DEFAULT '',
  body        TEXT NOT NULL,
  pinned      INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Bell tower: kind = event | work | alarm.
CREATE TABLE events (
  id          INTEGER PRIMARY KEY,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  title       TEXT NOT NULL,
  kind        TEXT NOT NULL DEFAULT 'event',
  starts_at   TEXT NOT NULL,
  ends_at     TEXT,
  place       TEXT NOT NULL DEFAULT '',
  notes       TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Market: a shop run somebody is making.
CREATE TABLE runs (
  id          INTEGER PRIMARY KEY,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  destination TEXT NOT NULL,
  cutoff_at   TEXT NOT NULL,
  notes       TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Market: something a house needs. state = open | taken | done.
CREATE TABLE needs (
  id          INTEGER PRIMARY KEY,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  text        TEXT NOT NULL,
  state       TEXT NOT NULL DEFAULT 'open',
  taken_by    INTEGER REFERENCES houses(id) ON DELETE SET NULL,
  run_id      INTEGER REFERENCES runs(id) ON DELETE SET NULL,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Market: something a house gives away. tag = giveaway | seeds | surplus | joint.
CREATE TABLE offers (
  id          INTEGER PRIMARY KEY,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  text        TEXT NOT NULL,
  tag         TEXT NOT NULL DEFAULT 'giveaway',
  state       TEXT NOT NULL DEFAULT 'open',
  claimed_by  INTEGER REFERENCES houses(id) ON DELETE SET NULL,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Watchtower: a house is away. watcher is an acknowledgment, never a tally.
CREATE TABLE away (
  id          INTEGER PRIMARY KEY,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  from_date   TEXT NOT NULL,
  to_date     TEXT NOT NULL,
  notes       TEXT NOT NULL DEFAULT '',
  watcher     INTEGER REFERENCES houses(id) ON DELETE SET NULL,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE settings (
  key    TEXT PRIMARY KEY,
  value  TEXT NOT NULL
);
