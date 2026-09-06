-- Quiet hours (21:00–07:00) stay the default, but they are the village's
-- habit, not a law. A phone may opt out of them. The flag sits on the DEVICE,
-- not the house: a phone that rings at 23:00 belongs to one person, and the
-- other phones of the same house have nothing to do with that choice.
-- Default 0 — silence unless a villager asked for the opposite.
ALTER TABLE devices ADD COLUMN quiet_ok INTEGER NOT NULL DEFAULT 0;

-- A sign-up used to carry one short note ("I bring the scythe"). The event's
-- comment thread does that job now and does it better, so the field goes.
-- What people already wrote is carried into that thread, not dropped: the
-- house keeps its crest on the line, the author name stays empty because a
-- sign-up note never had one.
INSERT INTO comments (subject, subject_id, house_id, author, body, created_at)
SELECT 'event', event_id, house_id, '', note, created_at
FROM event_signups WHERE trim(note) <> '';
ALTER TABLE event_signups DROP COLUMN note;
