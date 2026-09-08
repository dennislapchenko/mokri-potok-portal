-- The line a house wrote about where it lives goes (owner's decision
-- 2026-09-08): marking the parcel it lives on (house_homes, 015) does that job
-- on the map, and a sentence nobody else can act on was a second answer to
-- the same question.
ALTER TABLE houses DROP COLUMN about;
