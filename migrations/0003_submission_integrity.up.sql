-- Dedicated tamper-evidence snapshot for raw_event, independent of
-- source_key (migration 0002): when audit_id/audit_stage are present,
-- source_key is derived from that identity pair alone
-- (internal/submission.go, SourceKey) and does not cover raw_event
-- content -- so it cannot detect a raw_event payload changed out of band
-- while identity fields are left untouched. raw_event_sha256 closes that
-- gap; internal/traceability's chain verifier separately re-derives and
-- checks source_key for identity drift.
--
-- Not a GENERATED ALWAYS AS column: a generated column recomputes
-- automatically whenever raw_event changes, so an out-of-band UPDATE to
-- raw_event would silently update its own "proof" along with it instead
-- of producing a detectable mismatch -- exactly the tamper case this
-- column exists to catch.
--
-- Added nullable first, backfilled, then constrained -- the same pattern
-- migration 0002 used for source_key. The octet_length CHECK enforces the
-- digest shape at the database boundary, independent of any application
-- code path.
ALTER TABLE submissions ADD COLUMN raw_event_sha256 BYTEA;
UPDATE submissions SET raw_event_sha256 = sha256(raw_event) WHERE raw_event_sha256 IS NULL;
ALTER TABLE submissions ALTER COLUMN raw_event_sha256 SET NOT NULL;
ALTER TABLE submissions ADD CONSTRAINT submissions_raw_event_sha256_length
    CHECK (octet_length(raw_event_sha256) = 32);
