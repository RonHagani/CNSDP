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

// Submission is the durable record for one telemetry submission.
type Submission struct {
	ID     int64
	Status Status
	// RawEvent is the exact bytes of the received EventList item, stored
	// as BYTEA -- byte-for-byte, not a JSON-canonicalized re-serialization.
	// A queryable/semantic JSONB representation belongs later, in
	// normalized_events.content, not here.
	RawEvent   json.RawMessage
	AuditID    string
	AuditStage string
	CreatedAt  time.Time
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
		`SELECT id, status, raw_event, audit_id, audit_stage, created_at
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
		`SELECT id, status, raw_event, audit_id, audit_stage, created_at
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
		`SELECT id, status, raw_event, audit_id, audit_stage, created_at
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
	var status string
	if err := row.Scan(&s.ID, &status, &s.RawEvent, &s.AuditID, &s.AuditStage, &s.CreatedAt); err != nil {
		return nil, err
	}
	s.Status = Status(status)
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
// validation checkpoint's responsibility, not this one's.
//
// Admission is retry-safe: sourceKey derives a deterministic key from
// auditID+auditStage when both are available, or from a hash of rawEvent
// otherwise. A repeat delivery of the same source event -- same key, same
// raw_event, audit_id, and audit_stage -- resolves to the existing row's
// id rather than inserting a duplicate. If the same key arrives with any
// different content, the existing row is left unchanged and
// ErrSourceConflict is returned; this is a single race-safe statement, not
// a separate check-then-act (concurrent admissions of the same key are
// serialized by Postgres's own row lock on the conflicting row).
func Admit(ctx context.Context, db DB, rawEvent json.RawMessage, auditID, auditStage string) (int64, error) {
	key := sourceKey(rawEvent, auditID, auditStage)
	var id int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (source_key) DO UPDATE
		 SET raw_event = EXCLUDED.raw_event
		 WHERE submissions.raw_event = EXCLUDED.raw_event
		   AND submissions.audit_id = EXCLUDED.audit_id
		   AND submissions.audit_stage = EXCLUDED.audit_stage
		 RETURNING id`,
		[]byte(rawEvent), auditID, auditStage, key,
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

// sourceKey derives Admit's deterministic dedup key. The "id:"/"raw:"
// prefixes keep the two derivation schemes disjoint. The identity-based
// scheme canonically JSON-encodes the (auditID, auditStage) pair before
// hashing -- not a delimited concatenation -- because auditID/auditStage
// come from unconditionally-admitted, possibly-malformed items and cannot
// be assumed not to contain whatever separator a naive concatenation would
// pick; JSON's string escaping makes the encoding of the pair injective.
func sourceKey(rawEvent json.RawMessage, auditID, auditStage string) string {
	if auditID != "" && auditStage != "" {
		canonical, _ := json.Marshal([2]string{auditID, auditStage}) // marshaling a fixed-size []string cannot fail
		sum := sha256.Sum256(canonical)
		return "id:" + hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(rawEvent)
	return "raw:" + hex.EncodeToString(sum[:])
}
