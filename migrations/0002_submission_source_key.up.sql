-- submissions is the immutable source-record layer: it must preserve the
-- exact bytes of each EventList item as received, not a semantically-
-- equivalent re-serialization. Queryable/normalized JSON belongs at the
-- normalization stage (normalized_events.content, still JSONB), not here.
-- convert_to(..., 'UTF8') explicitly encodes the text to UTF-8 bytes. A
-- bare raw_event::text::bytea cast is NOT equivalent: casting text to
-- bytea is parsed through bytea's textual "escape" input format, where an
-- unescaped backslash (routine in JSON, e.g. \" or \u escapes) is
-- misinterpreted as an escape sequence rather than taken literally --
-- silently corrupting exactly the payloads this migration exists to
-- preserve. convert_to has no such ambiguity.
ALTER TABLE submissions
    ALTER COLUMN raw_event TYPE BYTEA USING convert_to(raw_event::text, 'UTF8');

-- Deterministic, persisted source-event key so a retried admission
-- attempt (e.g. a redelivered audit-webhook batch) resolves to the same
-- submission instead of creating a duplicate row. Derivation
-- (internal/submission.go, sourceKey): "id:" + sha256 of a canonical JSON
-- encoding of (auditID, auditStage) when both are available, else "raw:" +
-- sha256 of the exact raw item bytes.
--
-- Added nullable first, then backfilled, then constrained: no deployed
-- data is expected in normal operation, but any row that does already
-- exist when this migration runs (e.g. a migration-reversibility test
-- that seeds a row, steps down, then steps back up) must still be
-- backfillable rather than failing the NOT NULL constraint outright.
-- sha256(raw_event) is exactly sourceKey()'s own raw-hash fallback
-- scheme, applied uniformly here since no separate audit_id/audit_stage
-- split is available at migration time.
ALTER TABLE submissions ADD COLUMN source_key TEXT;
UPDATE submissions SET source_key = 'raw:' || encode(sha256(raw_event), 'hex') WHERE source_key IS NULL;
ALTER TABLE submissions ALTER COLUMN source_key SET NOT NULL;
ALTER TABLE submissions ADD CONSTRAINT submissions_source_key_key UNIQUE (source_key);
