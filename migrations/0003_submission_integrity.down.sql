-- Dropping the column also drops the octet_length CHECK constraint
-- defined on it (Postgres cascades a table constraint's removal when its
-- sole column is dropped), so no separate DROP CONSTRAINT is needed --
-- mirroring migration 0002's down migration, which drops source_key
-- without a separate step for its UNIQUE constraint.
ALTER TABLE submissions DROP COLUMN raw_event_sha256;
