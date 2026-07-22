// Package worker orchestrates the single-worker durable processing loop
// (ADR-0002): select the oldest eligible submission, dispatch it to
// exactly the one stage module that owns its current status, repeat. It
// owns no table itself and contains no stage business logic of its own —
// validateStage delegates classification entirely to internal/validation,
// and normalization dispatch delegates entirely to
// internal/normalization.Advance, which owns every read and write against
// normalized_events. The read-only join in oldestEligible is an
// orchestration-only exception to "no module reads another module's table
// directly": it never writes submissions or validation_outcomes and
// contains no validation business logic, only the claim predicate needed
// to select work. Stage handlers for evaluate, alert, and evidence, and
// every other workflow-stage module, are not implemented yet.
package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

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
		return validateStage(ctx, db, sub.ID, sub.RawEvent)
	case submission.StatusValidated:
		return normalization.Advance(ctx, db, sub)
	default:
		return fmt.Errorf("worker: claimed submission %d has unexpected status %q", sub.ID, sub.Status)
	}
}

// oldestEligible selects the single oldest submission this worker can
// currently advance, across two eligibility conditions:
//
//   - status = admitted (every admitted submission needs validation), or
//   - status = validated with a recorded valid outcome (FR-014: only a
//     valid submission proceeds to normalization; invalid, incomplete, and
//     unsupported submissions are permanently parked at validated with no
//     handler that ever claims them again).
//
// Ordering is by submission creation order (created_at), with id as the
// deterministic tie-breaker for rows created in the same instant. Neither
// stage is given blanket priority over the other: under sustained intake,
// an "admitted always wins" policy would starve eligible validated
// submissions indefinitely, so eligibility is decided by age across both
// conditions together, not by stage.
//
// This is a read-only join against validation_outcomes -- the table
// internal/validation owns -- solely to evaluate the claim predicate
// above; it never writes either table and contains no validation
// business logic beyond referencing the outcome vocabulary
// internal/validation already exports.
func oldestEligible(ctx context.Context, db *sql.DB) (*submission.Submission, error) {
	row := db.QueryRowContext(ctx,
		`SELECT s.id, s.status, s.raw_event, s.audit_id, s.audit_stage, s.created_at
		 FROM submissions s
		 LEFT JOIN validation_outcomes v ON v.submission_id = s.id
		 WHERE s.status = $1
		    OR (s.status = $2 AND v.outcome = $3)
		 ORDER BY s.created_at, s.id
		 LIMIT 1`,
		string(submission.StatusAdmitted), string(submission.StatusValidated), string(validation.OutcomeValid),
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

// validateStage implements the validation and classification module's
// worker-facing side (FR-004–FR-014): it classifies rawEvent, records the
// classification as a validation_outcomes row, and advances the submission
// from admitted to validated — unconditionally, regardless of which of the
// four outcomes was recorded. "validated" represents workflow progress
// through this stage, not a successful validation result (FR-014): a
// non-valid outcome still advances here and then stays parked at validated,
// since no later stage handler ever claims it. The insert and the status
// advance occur in one transaction (ADR-0002).
func validateStage(ctx context.Context, db *sql.DB, id int64, rawEvent json.RawMessage) error {
	result := validation.Classify(rawEvent)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("worker: begin validate stage tx for submission %d: %w", id, err)
	}
	defer tx.Rollback()

	var reason sql.NullString
	if result.Reason != "" {
		reason = sql.NullString{String: result.Reason, Valid: true}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO validation_outcomes (submission_id, outcome, reason) VALUES ($1, $2, $3)
		 ON CONFLICT (submission_id) DO NOTHING`,
		id, string(result.Outcome), reason,
	); err != nil {
		return fmt.Errorf("worker: insert validation outcome for submission %d: %w", id, err)
	}

	if err := submission.Advance(ctx, tx, id, submission.StatusAdmitted, submission.StatusValidated); err != nil {
		return fmt.Errorf("worker: advance submission %d to validated: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("worker: commit validate stage for submission %d: %w", id, err)
	}
	return nil
}
