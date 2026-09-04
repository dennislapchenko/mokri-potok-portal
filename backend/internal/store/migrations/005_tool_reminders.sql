-- When the holder of a tool was last reminded to bring it back. NULL = never.
ALTER TABLE tools ADD COLUMN reminded_at TEXT;
