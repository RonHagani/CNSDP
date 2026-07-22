ALTER TABLE submissions DROP COLUMN source_key;

-- convert_from(..., 'UTF8') decodes the stored bytes as text (unlike a
-- bare bytea::text cast, which would produce a hex/escape-formatted
-- representation per bytea_output, not the original text) before parsing
-- as JSON. Every admitted item was validated as parseable JSON before
-- insertion (it arrived as one element of a structurally-valid EventList
-- "items" array), so this reverse conversion is always structurally
-- possible.
--
-- This rollback is lossy: parsing back into JSONB preserves JSON
-- *semantics* (the same keys and values), but JSONB canonicalizes on
-- storage, so original whitespace and object-key ordering are NOT
-- preserved across a down-then-up round trip. That is acceptable only
-- because this repository is still pre-release and no deployed data
-- exists yet to lose fidelity on; it would not be an acceptable rollback
-- path once real admitted submissions exist.
ALTER TABLE submissions
    ALTER COLUMN raw_event TYPE JSONB USING convert_from(raw_event, 'UTF8')::jsonb;
