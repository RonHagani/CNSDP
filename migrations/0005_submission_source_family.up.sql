-- Source-family discriminator (ADR-0006): distinguishes which of the
-- platform's two approved source-event families a submission belongs to.
-- A constant default (unlike migrations 0002/0003's per-row computed
-- backfill) is sufficient and correct here: every row that exists before
-- this migration runs is, by construction, a Kubernetes submission -- v0.1
-- had no other source family until this migration's companion change
-- (0006) and the new CloudTrail intake endpoint it supports.
ALTER TABLE submissions ADD COLUMN source_family TEXT NOT NULL DEFAULT 'kubernetes'
    CHECK (source_family IN ('kubernetes', 'aws_cloudtrail'));
