-- Campground becomes a flow: a house notices a camper (arrived), a house says
-- it has the money (held), then hands it over (handed). Rows from before had
-- the noticing house holding the money by definition.
ALTER TABLE camp_takings ADD COLUMN held_by INTEGER REFERENCES houses(id) ON DELETE SET NULL;
ALTER TABLE camp_takings ADD COLUMN held_at TEXT;
UPDATE camp_takings SET held_by = house_id, held_at = created_at WHERE state IN ('held', 'handed');
