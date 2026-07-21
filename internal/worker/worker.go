// Package worker orchestrates the single-worker durable processing loop
// (ADR-0002): claim the oldest non-terminal submission, advance it exactly
// one stage, repeat. It owns no table itself and contains no stage
// business logic beyond this checkpoint's placeholder validate stage —
// each real stage's logic belongs to that stage's own module, invoked
// from here.
package worker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cnsdp/internal/submission"
)

// ErrNoWork is returned by ProcessOne when there is no non-terminal
// submission to advance.
var ErrNoWork = submission.ErrNoWork

// ProcessOne claims the oldest non-terminal submission and advances it
// exactly one stage. No SKIP LOCKED, no concurrency: this function
// assumes it is the only writer running against the database (ADR-0002) —
// a future need for concurrent workers requires its own claim-locking
// design, not an extension of this one.
func ProcessOne(ctx context.Context, db *sql.DB) error {
	sub, err := submission.OldestNonTerminal(ctx, db)
	if err != nil {
		if errors.Is(err, submission.ErrNoWork) {
			return ErrNoWork
		}
		return fmt.Errorf("worker: claim next submission: %w", err)
	}

	switch sub.Status {
	case submission.StatusAdmitted:
		return validateStage(ctx, db, sub.ID)
	default:
		return fmt.Errorf("worker: no stage handler implemented yet for status %q (submission %d)", sub.Status, sub.ID)
	}
}

// validateStage is this checkpoint's placeholder for the validation
// module (internal/validation does not exist yet): it records a single,
// always-valid outcome and advances the submission. A future checkpoint
// replaces this function's body with real four-way classification
// (FR-005) without changing its transaction shape — artifact insert and
// status advance, guarded, in one transaction (ADR-0002).
func validateStage(ctx context.Context, db *sql.DB, id int64) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("worker: begin validate stage tx for submission %d: %w", id, err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO validation_outcomes (submission_id, outcome, reason) VALUES ($1, 'valid', NULL)
		 ON CONFLICT (submission_id) DO NOTHING`,
		id,
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
