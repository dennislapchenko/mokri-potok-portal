-- One row per phone that allowed notifications. endpoint is the browser's push
-- address; p256dh/auth are the browser's keys for encrypting the payload.
CREATE TABLE push_subscriptions (
  id          INTEGER PRIMARY KEY,
  house_id    INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  device_id   INTEGER REFERENCES devices(id) ON DELETE SET NULL,
  endpoint    TEXT NOT NULL UNIQUE,
  p256dh      TEXT NOT NULL,
  auth        TEXT NOT NULL,
  created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Kinds a house switched OFF. Empty = everything on (the default).
-- kind: posts | needs | offers | runs | events | away
CREATE TABLE notify_off (
  house_id  INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  kind      TEXT NOT NULL,
  PRIMARY KEY (house_id, kind)
);
