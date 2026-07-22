// Package worker orchestrates the single-worker durable processing loop
// (ADR-0002): select the oldest eligible submission, dispatch it to
// exactly the one stage module that owns its current status, repeat. It
// owns no table itself and contains no stage business logic of its own —
// dispatch delegates entirely to each stage module's own Advance
// function (internal/validation, internal/normalization,
// internal/detection, internal/alerting), each of which owns every read
// and write against its own artifact table. The read-only join in
// oldestEligible is an orchestration-only exception to "no module reads
// another module's table directly": it never writes any table and
// contains no stage business logic, only the claim predicate needed to
// select work. Stage handlers for evidence, and every other
// workflow-stage module beyond alerting, are not implemented yet.
package worker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cnsdp/internal/alerting"
	"cnsdp/internal/detection"
	"cnsdp/internal/normalization"
	"cnsdp/internal/submission"
	"cnsdp/internal/validation"
)

// ErrNoWork is returned by ProcessOne when there is no submission at a
// status this worker knows how to advance.
var ErrNoWork = submission.ErrNoWork

// ProcessOne selects the oldest eligible submission across every stage
// this worker knows how to advance and dispatches it to exactly one
// stage. No SKIP LOCKED, no concurrency: this function assumes it is the
// only writer running against the database (ADR-0002) — a future need for
// concurrent workers requires its own claim-locking design, not an
// extension of this one.
func ProcessOne(ctx context.Context, db *sql.DB) error {
	sub, err := oldestEligible(ctx, db)
	if err != nil {
		if errors.Is(err, submission.ErrNoWork) {
			return ErrNoWork
		}
		return fmt.Errorf("worker: claim next eligible submission: %w", err)
	}

	switch sub.Status {
	case submission.StatusAdmitted:
		return validation.Advance(ctx, db, sub)
	case submission.StatusValidated:
		return normalization.Advance(ctx, db, sub)
	case submission.StatusNormalized:
		return detection.Advance(ctx, db, sub)
	case submission.StatusEvaluated:
		return alerting.Advance(ctx, db, sub)
	default:
		return fmt.Errorf("worker: claimed submission %d has unexpected status %q", sub.ID, sub.Status)
	}
}

// oldestEligible selects the single oldest submission this worker can
// currently advance, across four eligibility conditions:
//
//   - status = admitted (every admitted submission needs validation),
//   - status = validated with a recorded valid outcome (FR-014: only a
//     valid submission proceeds to normalization; invalid, incomplete, and
//     unsupported submissions are permanently parked at validated with no
//     handler that ever claims them again),
//   - status = normalized (every normalized submission needs detection
//     evaluation; no outcome gate applies here since normalization
//     unconditionally produces one normalized event per valid submission), or
//   - status = evaluated (every evaluated submission needs alert
//     generation; no outcome gate applies here either -- the
//     evaluated -> alerted transition is unconditional, since a
//     submission with zero matching detection results still completes
//     alert-generation processing, just with zero alerts produced).
//
// Ordering is by submission creation order (created_at), with id as the
// deterministic tie-breaker for rows created in the same instant. No
// stage is given blanket priority over another: under sustained intake,
// an "admitted always wins" policy would starve eligible validated,
// normalized, or evaluated submissions indefinitely, so eligibility is
// decided by age across all four conditions together, not by stage.
//
// This is a read-only join against validation_outcomes -- a table
// internal/validation owns -- solely to evaluate the claim predicate
// above; it never writes any table and contains no stage business logic
// beyond referencing the outcome vocabulary internal/validation already
// exports.
func oldestEligible(ctx context.Context, db *sql.DB) (*submission.Submission, error) {
	row := db.QueryRowContext(ctx,
		`SELECT s.id, s.status, s.raw_event, s.audit_id, s.audit_stage, s.created_at
		 FROM submissions s
		 LEFT JOIN validation_outcomes v ON v.submission_id = s.id
		 WHERE s.status = $1
		    OR (s.status = $2 AND v.outcome = $3)
		    OR s.status = $4
		    OR s.status = $5
		 ORDER BY s.created_at, s.id
		 LIMIT 1`,
		string(submission.StatusAdmitted), string(submission.StatusValidated), string(validation.OutcomeValid),
		string(submission.StatusNormalized), string(submission.StatusEvaluated),
	)

	var sub submission.Submission
	var status string
	if err := row.Scan(&sub.ID, &status, &sub.RawEvent, &sub.AuditID, &sub.AuditStage, &sub.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, submission.ErrNoWork
		}
		return nil, fmt.Errorf("worker: select oldest eligible submission: %w", err)
	}
	sub.Status = submission.Status(status)
	return &sub, nil
}
