-- A house that lives on somebody else's land marks that parcel instead of
-- being given it. Owner's decision 2026-09-07: the map should show where such
-- a house lives, and the Houses room should say whose land it is — but the
-- land stays the owner's, so this can never read as a second parcel list.
--
-- Deliberately not `house_parcels`: that table is keyed by parcel because a
-- parcel has one owner. A parcel can hold any number of houses.
CREATE TABLE house_homes (
  house_id  INTEGER NOT NULL REFERENCES houses(id) ON DELETE CASCADE,
  parcel    TEXT NOT NULL,
  PRIMARY KEY (house_id, parcel)
);
