-- Spike-scoped schema. Not a product schema; column names, types, and the
-- traceability-link representation are simplified for the purpose of this
-- spike only (see FINDINGS.md).

CREATE TABLE submissions (
    id SERIAL PRIMARY KEY,
    status TEXT NOT NULL DEFAULT 'admitted',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE detection_definitions (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    revision TEXT NOT NULL
);

-- One validation outcome per submission.
CREATE TABLE validation_outcomes (
    id SERIAL PRIMARY KEY,
    submission_id INTEGER NOT NULL REFERENCES submissions(id),
    outcome TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (submission_id)
);

-- One normalized event per submission.
CREATE TABLE normalized_events (
    id SERIAL PRIMARY KEY,
    submission_id INTEGER NOT NULL REFERENCES submissions(id),
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (submission_id)
);

-- One detection result per (submission, detection-definition revision).
CREATE TABLE detection_results (
    id SERIAL PRIMARY KEY,
    submission_id INTEGER NOT NULL REFERENCES submissions(id),
    detection_definition_id INTEGER NOT NULL REFERENCES detection_definitions(id),
    matched BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (submission_id, detection_definition_id)
);

-- One alert per matching detection result.
CREATE TABLE alerts (
    id SERIAL PRIMARY KEY,
    detection_result_id INTEGER NOT NULL REFERENCES detection_results(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (detection_result_id)
);

-- One traceability link per (source, target, relation).
CREATE TABLE traceability_links (
    id SERIAL PRIMARY KEY,
    source_ref TEXT NOT NULL,
    target_ref TEXT NOT NULL,
    relation TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (source_ref, target_ref, relation)
);

INSERT INTO detection_definitions (name, revision) VALUES
    ('scenario-stub-a', 'rev1'),
    ('scenario-stub-b', 'rev1');
