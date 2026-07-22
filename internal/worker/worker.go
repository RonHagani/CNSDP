// Package worker orchestrates the single-worker durable processing loop
// (ADR-0002): claim a submission at a stage-specific status, advance it
// exactly one stage, repeat. It owns no table itself and contains no stage
// business logic beyond invoking each stage's own module — validateStage
// delegates classification entirely to internal/validation. Stage handlers
// for normalize, evaluate, alert, and evidence, and every other
// workflow-stage module, are not implemented yet.
package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"cnsdp/internal/submission"
	"cnsdp/internal/validation"
)

// ErrNoWork is returned by ProcessOne when there is no submission at a
// status this worker knows how to advance.
var ErrNoWork = submission.ErrNoWork

// ProcessOne claims the oldest submission currently at the admitted status
// and advances it exactly one stage. No SKIP LOCKED, no concurrency: this
// function assumes it is the only writer running against the database
// (ADR-0002) — a future need for concurrent workers requires its own
// claim-locking design, not an extension of this one.
//
// Claiming is stage-specific (OldestAtStatus), not merely "any non-terminal
// submission" (OldestNonTerminal): a submission classified invalid,
// incomplete, or unsupported advances to validated and is permanently
// parked there (FR-014) with no further handler to claim it. Using
// OldestNonTerminal here would have the worker repeatedly reclaim the same
// parked submission forever, starving every submission behind it.
func ProcessOne(ctx context.Context, db *sql.DB) error {
	sub, err := submission.OldestAtStatus(ctx, db, submission.StatusAdmitted)
	if err != nil {
		if errors.Is(err, submission.ErrNoWork) {
			return ErrNoWork
		}
		return fmt.Errorf("worker: claim next admitted submission: %w", err)
	}
	return validateStage(ctx, db, sub.ID, sub.RawEvent)
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
