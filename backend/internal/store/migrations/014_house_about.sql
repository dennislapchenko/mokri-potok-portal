-- Option F, decided by the owner 2026-09-06: a member who lives here without
-- land is a house whose parcel list is empty. No new kind of row, so nothing
-- in the app can rank them below a landholder.
--
-- What replaces the land on such a row is a line the house writes about
-- itself — "v koči ob potoku". Every house may write one, so it is not a badge
-- and not a marker of who owns what. It is also what the Watchtower shows
-- beside a house's absence, so the neighbour whose building stands empty is
-- not surprised by the notice.
ALTER TABLE houses ADD COLUMN about TEXT NOT NULL DEFAULT '';
