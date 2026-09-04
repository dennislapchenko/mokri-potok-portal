-- Tool shed: what the village lends. The owner house keeps the row; held_by is
-- whoever has it right now. Same shape as an offer, but a tool comes back.
CREATE TABLE tools (
  id          INTEGER PRIMARY KEY,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  notes       TEXT NOT NULL DEFAULT '',
  held_by     INTEGER REFERENCES houses(id) ON DELETE SET NULL,
  held_since  TEXT,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Work-bee sign-ups. Any event can take them, not only kind='work'.
CREATE TABLE event_signups (
  event_id    INTEGER NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  note        TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (event_id, house_id)
);

-- Pairing codes: a logged-in house adds one more phone without the steward.
-- Six digits, single use, minutes long — an iPhone home-screen app gets its own
-- storage, so installing after logging in leaves the new icon logged out.
CREATE TABLE pairings (
  code        TEXT PRIMARY KEY,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  expires_at  TEXT NOT NULL,
  used_at     TEXT,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);
