-- The map's data lives in the database like the photos do, so the SQLite
-- backup is the whole village and one image serves any village. Two rows:
-- 'parcels' (a GeoJSON FeatureCollection, planar metres) and 'water' (the
-- modelled watercourses). A village with no 'parcels' row has no map yet, and
-- the page says so. source and snapshot are the caption under the map, typed
-- at import; water carries neither.
CREATE TABLE map_files (
  name       TEXT PRIMARY KEY,
  bytes      BLOB NOT NULL,
  source     TEXT NOT NULL DEFAULT '',
  snapshot   TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
