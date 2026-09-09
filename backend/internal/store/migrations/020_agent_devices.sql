-- A key an agent holds is a device of the house: same token, same hash, same
-- list, same Remove button. The flag is what the Houses room shows (🤖, not 📱)
-- and what requireHouse reads: an agent device opens POST /api/mcp and nothing
-- else, and is never a steward whatever its house is.
ALTER TABLE devices ADD COLUMN agent INTEGER NOT NULL DEFAULT 0;
