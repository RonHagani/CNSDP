-- Walking-skeleton minimum schema (ARCH-01 §4, ADR-0002).
--
-- Relational-integrity rule applied throughout: an artifact chain
-- (alert -> detection_result -> normalized_event -> submission) has exactly
-- one path through this schema. No table stores more than one independent
-- foreign key into that chain, so a cross-submission mismatch is not
-- merely rejected by a constraint -- it has no column to be expressed in.

CREATE TABLE submissions (
    id           BIGSERIAL PRIMARY KEY,
    status       TEXT NOT NULL DEFAULT 'admitted'
                 CHECK (status IN ('admitted', 'validated', 'normalized', 'evaluated', 'alerted', 'evidenced')),
    raw_event    JSONB NOT NULL,
    audit_id     TEXT NOT NULL,
    audit_stage  TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE validation_outcomes (
    id             BIGSERIAL PRIMARY KEY,
    submission_id  BIGINT NOT NULL REFERENCES submissions(id),
    outcome        TEXT NOT NULL
                   CHECK (outcome IN ('valid', 'invalid', 'incomplete', 'unsupported')),
    reason         TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (submission_id)
);

CREATE TABLE normalized_events (
    id                       BIGSERIAL PRIMARY KEY,
    submission_id            BIGINT NOT NULL REFERENCES submissions(id),
    content                  JSONB NOT NULL,
    representation_revision  TEXT NOT NULL,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (submission_id)
);

CREATE TABLE detection_definitions (
    id          BIGSERIAL PRIMARY KEY,
    scenario    TEXT NOT NULL,
    revision    TEXT NOT NULL,
    content     JSONB NOT NULL,
    loaded_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (scenario, revision)
);

-- Submission is deliberately NOT stored here. It is derived through
-- normalized_event_id -> normalized_events.submission_id: a single path,
-- so it cannot disagree with itself. detection_definition_id is a
-- reference to a global, submission-independent definition and carries no
-- chain-consistency risk.
CREATE TABLE detection_results (
    id                       BIGSERIAL PRIMARY KEY,
    normalized_event_id      BIGINT NOT NULL REFERENCES normalized_events(id),
    detection_definition_id  BIGINT NOT NULL REFERENCES detection_definitions(id),
    matched                  BOOLEAN NOT NULL,
    match_reason             JSONB,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Equivalent to "one per submission and detection-definition revision"
    -- (ADR-0002): normalized_events.UNIQUE(submission_id) already makes
    -- normalized_event_id <-> submission_id a 1:1 mapping.
    UNIQUE (normalized_event_id, detection_definition_id),
    CHECK ( (matched AND match_reason IS NOT NULL) OR (NOT matched AND match_reason IS NULL) )
);

CREATE TABLE alerts (
    id                    BIGSERIAL PRIMARY KEY,
    detection_result_id  BIGINT NOT NULL REFERENCES detection_results(id),
    summary               JSONB NOT NULL,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (detection_result_id)
);

-- No traceability_links table: with the chain above fully and uniquely
-- derivable through enforced foreign keys, a separate persisted copy would
-- itself be the kind of redundant, independently-mutable relationship this
-- review exists to eliminate. The chain is walked with a join
-- (alerts -> detection_results -> normalized_events -> submissions); see
-- migrations/0001_initial_schema.down.sql and the integration tests for
-- the corresponding verification queries.
