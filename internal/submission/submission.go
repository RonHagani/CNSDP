// Package submission owns every read and write against the submissions
// table (ARCH-01 §2, §3) — the durable record of each telemetry
// submission's processing stage. No other package writes this table.
package submission

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Status is a submission's processing stage. These six values are the
// only ones the submissions.status CHECK constraint
// (migrations/0001_initial_schema.up.sql) permits — nothing outside this
// package should declare its own copy of this vocabulary.
type Status string

const (
	StatusAdmitted   Status = "admitted"
	StatusValidated  Status = "validated"
	StatusNormalized Status = "normalized"
	StatusEvaluated  Status = "evaluated"
	StatusAlerted    Status = "alerted"
	StatusEvidenced  Status = "evidenced"
)

// Family identifies which of the platform's two approved source-event
// families (ADR-0006) a submission belongs to -- the only two values the
// submissions.source_family CHECK constraint
// (migrations/0005_submission_source_family.up.sql) permits. Exactly two
// families are supported in v0.1 (docs/scope.md); this is a fixed
// enumeration, not an extensible plugin registry.
type Family string

const (
	FamilyKubernetes Family = "kubernetes"
	FamilyCloudTrail Family = "aws_cloudtrail"
)

// Submission is the durable record for one telemetry submission.
type Submission struct {
	ID     int64
	Status Status
	// RawEvent is the exact bytes of the received item, stored as BYTEA --
	// byte-for-byte, not a JSON-canonicalized re-serialization. A
	// queryable/semantic JSONB representation belongs later, in
	// normalized_events.content, not here.
	RawEvent json.RawMessage
	// AuditID and AuditStage are Kubernetes-specific (the auditID and audit
	// stage of a Kubernetes audit.k8s.io/v1 Event, FR-002/FR-007) and are
	// always empty for a SourceFamily == FamilyCloudTrail submission --
	// CloudTrail has no audit-stage concept and its own recorded identity
	// (eventID) is carried by SourceEventID instead, not by these fields.
	AuditID    string
	AuditStage string
	// SourceFamily is the source-event family this submission belongs to
	// (ADR-0006). SourceEventID is the AWS CloudTrail record's own
	// recorded identity (eventID, FR-002(a)/FR-007(a) for that family) and
	// is always empty for a SourceFamily == FamilyKubernetes submission.
	SourceFamily  Family
	SourceEventID string
	CreatedAt     time.Time
}

var (
	// ErrNotFound is returned when a submission id does not exist.
	ErrNotFound = errors.New("submission: not found")

	// ErrStatusConflict is returned by Advance when the persisted status
	// no longer matches the expected source status — either the
	// transition already happened (a stale or retried caller) or the
	// caller's assumption about the submission's stage was wrong. It is
	// never silently reapplied.
	ErrStatusConflict = errors.New("submission: status conflict")

	// ErrNoWork is returned by OldestNonTerminal when every submission
	// has already reached the terminal (evidenced) status.
	ErrNoWork = errors.New("submission: no non-terminal submission available")

	// ErrSourceConflict is returned by Admit when a submission with the
	// same derived source_key already exists but its persisted content
	// (raw_event, audit_id, audit_stage) does not match this call's
	// arguments -- so this is not a safe retry of the same event, and
	// neither updating the existing row nor inserting a new one is safe.
	ErrSourceConflict = errors.New("submission: source conflict")
)

// DB is the minimal subset of *sql.DB / *sql.Tx this package needs, so a
// caller can pass either a plain connection or an already-open
// transaction. Passing an open *sql.Tx to Advance is how a stage's
// artifact insert and its status advance are made atomic (ADR-0002).
type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Get retrieves a submission by id. Returns ErrNotFound if it does not
// exist, or a wrapped error for any other database failure.
func Get(ctx context.Context, db DB, id int64) (*Submission, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, status, raw_event, audit_id, audit_stage, source_family, source_event_id, created_at
		 FROM submissions WHERE id = $1`,
		id)
	s, err := scanSubmission(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("submission: get %d: %w", id, err)
	}
	return s, nil
}

// OldestNonTerminal returns the oldest submission (by id) whose status is
// not yet evidenced — the single-worker claim query confirmed by Spike 2
// (spikes/02-postgres-durable-worker/FINDINGS.md, "Observed results").
// Returns ErrNoWork if every submission has already reached evidenced.
func OldestNonTerminal(ctx context.Context, db DB) (*Submission, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, status, raw_event, audit_id, audit_stage, source_family, source_event_id, created_at
		 FROM submissions WHERE status <> $1 ORDER BY id LIMIT 1`,
		string(StatusEvidenced))
	s, err := scanSubmission(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoWork
	}
	if err != nil {
		return nil, fmt.Errorf("submission: select oldest non-terminal: %w", err)
	}
	return s, nil
}

// OldestAtStatus returns the oldest submission (by id) currently at exactly
// the given status. Unlike OldestNonTerminal, a status is not "claimable"
// merely for being non-terminal: a stage's worker handler only knows how to
// advance a submission from the one status it owns, and some statuses (e.g.
// validated with a non-valid recorded outcome) are permanently parked short
// of evidenced (FR-014) without any handler ever claiming them again.
// Returns ErrNoWork if no submission is currently at that status.
func OldestAtStatus(ctx context.Context, db DB, status Status) (*Submission, error) {
	row := db.QueryRowContext(ctx,
		`SELECT id, status, raw_event, audit_id, audit_stage, source_family, source_event_id, created_at
		 FROM submissions WHERE status = $1 ORDER BY id LIMIT 1`,
		string(status))
	s, err := scanSubmission(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoWork
	}
	if err != nil {
		return nil, fmt.Errorf("submission: select oldest at status %s: %w", status, err)
	}
	return s, nil
}

func scanSubmission(row *sql.Row) (*Submission, error) {
	var s Submission
	var status, family string
	if err := row.Scan(&s.ID, &status, &s.RawEvent, &s.AuditID, &s.AuditStage, &family, &s.SourceEventID, &s.CreatedAt); err != nil {
		return nil, err
	}
	s.Status = Status(status)
	s.SourceFamily = Family(family)
	return &s, nil
}

// Advance moves a submission from exactly the expected source status to
// the target status. The update is guarded: it succeeds only if the
// persisted status still equals from at the moment of the update, so a
// stale, already-applied, or otherwise unexpected transition is rejected
// (ErrStatusConflict) rather than silently reapplied or overwritten.
//
// Pass an open *sql.Tx as db to make this atomic with a stage's own
// artifact insert, in the same transaction (ADR-0002).
func Advance(ctx context.Context, db DB, id int64, from, to Status) error {
	res, err := db.ExecContext(ctx,
		`UPDATE submissions SET status = $1 WHERE id = $2 AND status = $3`,
		string(to), id, string(from))
	if err != nil {
		return fmt.Errorf("submission: advance %d from %s to %s: %w", id, from, to, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("submission: advance %d: rows affected: %w", id, err)
	}
	if n == 1 {
		return nil
	}

	// Zero rows affected: determine whether the submission doesn't exist
	// at all, or exists with a status other than the expected source.
	current, getErr := Get(ctx, db, id)
	if errors.Is(getErr, ErrNotFound) {
		return ErrNotFound
	}
	if getErr != nil {
		return fmt.Errorf("submission: advance %d: determine conflict cause: %w", id, getErr)
	}
	return fmt.Errorf("%w: submission %d expected status %q, found %q", ErrStatusConflict, id, from, current.Status)
}

// Admit durably records a newly received telemetry submission at the
// defined intake (FR-001, FR-003) in its initial admitted status (the
// column default). Every syntactically-identifiable item is admitted
// regardless of content shape -- FR-005's four-way classification is the
// validation checkpoint's responsibility, not this one's. family
// identifies which of the two approved source-event families (ADR-0006)
// this submission belongs to; auditID/auditStage are Kubernetes-specific
// and sourceEventID is CloudTrail-specific -- exactly one pair is
// meaningful for a given family, and the caller passes "" for the other
// family's fields (see Submission's field docs).
//
// Admission is retry-safe: SourceKey derives a deterministic key from the
// family-appropriate identity fields when available, or from a hash of
// family and rawEvent otherwise. A repeat delivery of the same source event -- same
// key, same raw_event, audit_id, audit_stage, source_family, and
// source_event_id -- resolves to the existing row's id rather than
// inserting a duplicate. If the same key arrives with any different
// content, the existing row is left unchanged and ErrSourceConflict is
// returned; this is a single race-safe statement, not a separate
// check-then-act (concurrent admissions of the same key are serialized by
// Postgres's own row lock on the conflicting row).
func Admit(ctx context.Context, db DB, rawEvent json.RawMessage, family Family, auditID, auditStage, sourceEventID string) (int64, error) {
	key := SourceKey(rawEvent, family, auditID, auditStage, sourceEventID)
	digest := sha256.Sum256(rawEvent)
	var id int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_family, source_event_id, source_key, raw_event_sha256)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (source_key) DO UPDATE
		 SET raw_event = EXCLUDED.raw_event
		 WHERE submissions.raw_event = EXCLUDED.raw_event
		   AND submissions.audit_id = EXCLUDED.audit_id
		   AND submissions.audit_stage = EXCLUDED.audit_stage
		   AND submissions.source_family = EXCLUDED.source_family
		   AND submissions.source_event_id = EXCLUDED.source_event_id
		 RETURNING id`,
		[]byte(rawEvent), auditID, auditStage, string(family), sourceEventID, key, digest[:],
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		// The conflicting row exists but its content doesn't match --
		// Postgres left it untouched (the WHERE predicate failed) and
		// RETURNING produced nothing.
		return 0, fmt.Errorf("%w: source key %s already used by a submission with different content", ErrSourceConflict, key)
	}
	if err != nil {
		return 0, fmt.Errorf("submission: admit: %w", err)
	}
	return id, nil
}

// SourceKey derives Admit's deterministic dedup key. The "id:"/"ct:"/"raw:"
// prefixes keep the three derivation schemes disjoint:
//
//   - FamilyCloudTrail with a non-empty sourceEventID hashes the eventID
//     alone ("ct:") -- CloudTrail's own recorded event identity (FR-002(a),
//     ADR-0006 Consequences).
//   - Any other family (FamilyKubernetes, or unset for backward
//     compatibility) with both auditID and auditStage present canonically
//     JSON-encodes the pair before hashing ("id:") -- not a delimited
//     concatenation -- because these come from unconditionally-admitted,
//     possibly-malformed items and cannot be assumed not to contain
//     whatever separator a naive concatenation would pick; JSON's string
//     escaping makes the encoding of the pair injective.
//   - Otherwise, a hash of family and the raw event bytes together ("raw:").
//     family is included so that two submissions from different families
//     that both lack any extractable identity can never derive the same
//     key merely for sharing byte-identical raw content -- without this,
//     a Kubernetes and a CloudTrail submission with identical malformed
//     bodies would collide on source_key despite being unrelated events.
//     A single NUL byte separates family from rawEvent: family is always
//     one of a small set of plain lowercase identifiers (FamilyKubernetes,
//     FamilyCloudTrail) that cannot themselves contain a NUL byte, so the
//     boundary between the two is unambiguous without JSON-style escaping.
//
// Exported so internal/traceability can re-derive the same key from a
// submission's currently stored identity fields and compare it against
// the persisted source_key column, to detect out-of-band identity drift
// (NFR-017, AC-015) -- source_key itself is never used as a raw_event
// integrity signal: in the "id:"/"ct:" branches it does not cover
// raw_event content at all (see migrations/0003_submission_integrity).
func SourceKey(rawEvent json.RawMessage, family Family, auditID, auditStage, sourceEventID string) string {
	switch family {
	case FamilyCloudTrail:
		if sourceEventID != "" {
			sum := sha256.Sum256([]byte(sourceEventID))
			return "ct:" + hex.EncodeToString(sum[:])
		}
	default:
		if auditID != "" && auditStage != "" {
			canonical, _ := json.Marshal([2]string{auditID, auditStage}) // marshaling a fixed-size []string cannot fail
			sum := sha256.Sum256(canonical)
			return "id:" + hex.EncodeToString(sum[:])
		}
	}
	namespaced := append([]byte(string(family)+"\x00"), rawEvent...)
	sum := sha256.Sum256(namespaced)
	return "raw:" + hex.EncodeToString(sum[:])
}

// List returns up to limit submissions ordered by id ascending with
// id > after -- a keyset ("seek") page, not an OFFSET-based one. Keyset
// pagination is index-only against the primary key and stays correct
// while the worker concurrently inserts and advances submissions, unlike
// OFFSET, which can skip or duplicate rows under concurrent writes at the
// volumes AC-021 exercises (10,000+ accumulated submissions). Pass after=0
// for the first page. If status is non-nil, only submissions currently at
// that status are returned -- the sanctioned way a caller filters to
// exactly the "not yet validated" submissions (status == StatusAdmitted),
// since a validation_outcomes row and the admitted -> validated transition
// are always written together in one transaction (internal/validation.Advance):
// status == StatusAdmitted if and only if no validation outcome has been
// recorded yet. An empty result returns a nil slice and a nil error, never
// an error.
func List(ctx context.Context, db *sql.DB, after int64, status *Status, limit int) ([]Submission, error) {
	var rows *sql.Rows
	var err error
	if status != nil {
		rows, err = db.QueryContext(ctx,
			`SELECT id, status, raw_event, audit_id, audit_stage, source_family, source_event_id, created_at
			 FROM submissions WHERE id > $1 AND status = $2 ORDER BY id ASC LIMIT $3`,
			after, string(*status), limit)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT id, status, raw_event, audit_id, audit_stage, source_family, source_event_id, created_at
			 FROM submissions WHERE id > $1 ORDER BY id ASC LIMIT $2`,
			after, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("submission: list: %w", err)
	}
	defer rows.Close()

	var out []Submission
	for rows.Next() {
		s, err := scanSubmissionRows(rows)
		if err != nil {
			return nil, fmt.Errorf("submission: list: scan: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("submission: list: %w", err)
	}
	return out, nil
}

// GetMany batch-fetches every submission whose id is in ids, ordered by id
// ascending, in exactly one query -- the bounded-query-count counterpart to
// List for a caller (internal/retrieval's submissions list, for an
// outcome-filtered page) that already knows a page's ids from
// internal/validation's own sanctioned read and needs the matching
// submission rows without querying once per id. A requested id with no
// matching row (never expected in ordinary operation: submissions are
// never deleted, PC-P-... deployment-lifetime retention) is simply absent
// from the result rather than an error. An empty ids slice returns a nil
// slice and a nil error without issuing a query.
func GetMany(ctx context.Context, db *sql.DB, ids []int64) ([]Submission, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := db.QueryContext(ctx,
		`SELECT id, status, raw_event, audit_id, audit_stage, source_family, source_event_id, created_at
		 FROM submissions WHERE id = ANY($1) ORDER BY id ASC`,
		ids)
	if err != nil {
		return nil, fmt.Errorf("submission: get many: %w", err)
	}
	defer rows.Close()

	var out []Submission
	for rows.Next() {
		s, err := scanSubmissionRows(rows)
		if err != nil {
			return nil, fmt.Errorf("submission: get many: scan: %w", err)
		}
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("submission: get many: %w", err)
	}
	return out, nil
}

// scanSubmissionRows scans one row of a *sql.Rows result set produced by
// List or GetMany -- the same column order as scanSubmission's single-row
// *sql.Row, factored out so both bulk readers share one scan path.
func scanSubmissionRows(rows *sql.Rows) (Submission, error) {
	var s Submission
	var status, family string
	if err := rows.Scan(&s.ID, &status, &s.RawEvent, &s.AuditID, &s.AuditStage, &family, &s.SourceEventID, &s.CreatedAt); err != nil {
		return Submission{}, err
	}
	s.Status = Status(status)
	s.SourceFamily = Family(family)
	return s, nil
}

// Count reports the total number of submissions, optionally restricted to
// those currently at status (nil means every submission regardless of
// status) -- the total a paginated list reports alongside a page, computed
// independently of that page's own LIMIT.
func Count(ctx context.Context, db *sql.DB, status *Status) (int64, error) {
	var n int64
	var err error
	if status != nil {
		err = db.QueryRowContext(ctx, `SELECT count(*) FROM submissions WHERE status = $1`, string(*status)).Scan(&n)
	} else {
		err = db.QueryRowContext(ctx, `SELECT count(*) FROM submissions`).Scan(&n)
	}
	if err != nil {
		return 0, fmt.Errorf("submission: count: %w", err)
	}
	return n, nil
}

// AdmittedSummary reports the total number of submissions ever admitted
// and the created_at of the most recently admitted one (nil if none exist)
// -- the two facts internal/datasources projects onto the platform's one
// ingestion channel (FR-036). It is a retrospective count, never a health
// or delivery judgment.
func AdmittedSummary(ctx context.Context, db DB) (count int64, lastAdmittedAt *time.Time, err error) {
	var last sql.NullTime
	if err := db.QueryRowContext(ctx, `SELECT count(*), max(created_at) FROM submissions`).Scan(&count, &last); err != nil {
		return 0, nil, fmt.Errorf("submission: admitted summary: %w", err)
	}
	if last.Valid {
		lastAdmittedAt = &last.Time
	}
	return count, lastAdmittedAt, nil
}
