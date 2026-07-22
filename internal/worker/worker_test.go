//go:build integration

package worker

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
)

func seedAdmitted(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key) VALUES ('{}', 'a', 'ResponseComplete', $1) RETURNING id`,
		testutil.UniqueKey(t),
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed admitted submission: %v", err)
	}
	return id
}

func countValidationOutcomes(t *testing.T, db *sql.DB, submissionID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM validation_outcomes WHERE submission_id = $1`, submissionID,
	).Scan(&n); err != nil {
		t.Fatalf("count validation_outcomes: %v", err)
	}
	return n
}

func TestValidateStage_AdvancesToValidatedWithOneOutcome(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	id := seedAdmitted(t, db)

	if err := validateStage(context.Background(), db, id); err != nil {
		t.Fatalf("validateStage: %v", err)
	}

	got, err := submission.Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusValidated {
		t.Errorf("status = %q, want %q", got.Status, submission.StatusValidated)
	}
	if n := countValidationOutcomes(t, db, id); n != 1 {
		t.Errorf("validation_outcomes rows = %d, want 1", n)
	}

	var outcome string
	var reason sql.NullString
	if err := db.QueryRowContext(context.Background(),
		`SELECT outcome, reason FROM validation_outcomes WHERE submission_id = $1`, id,
	).Scan(&outcome, &reason); err != nil {
		t.Fatalf("read validation outcome: %v", err)
	}
	if outcome != "valid" {
		t.Errorf("outcome = %q, want %q", outcome, "valid")
	}
	if reason.Valid {
		t.Errorf("reason = %q, want NULL for a valid outcome", reason.String)
	}
}

func TestValidateStage_RetryAfterSuccessDoesNotDuplicate(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	id := seedAdmitted(t, db)

	if err := validateStage(context.Background(), db, id); err != nil {
		t.Fatalf("first validateStage: %v", err)
	}

	// Simulate a retry against the same, now already-validated submission.
	err := validateStage(context.Background(), db, id)
	if !errors.Is(err, submission.ErrStatusConflict) {
		t.Fatalf("expected retry to fail with ErrStatusConflict, got %v", err)
	}

	if n := countValidationOutcomes(t, db, id); n != 1 {
		t.Errorf("validation_outcomes rows after retry = %d, want still 1 (no duplicate)", n)
	}
	got, err := submission.Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusValidated {
		t.Errorf("status = %q, want %q (unchanged by the rejected retry)", got.Status, submission.StatusValidated)
	}
}

func TestProcessOne_AdvancesOldestAdmittedSubmission(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	id := seedAdmitted(t, db)

	if err := ProcessOne(context.Background(), db); err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}

	got, err := submission.Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusValidated {
		t.Errorf("status = %q, want %q", got.Status, submission.StatusValidated)
	}
}

func TestProcessOne_NoWorkWhenNothingPending(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	err := ProcessOne(context.Background(), db)
	if !errors.Is(err, ErrNoWork) {
		t.Fatalf("expected ErrNoWork, got %v", err)
	}
}

// TestValidateStage_FailureBetweenInsertAndAdvanceRollsBackTransaction
// forces a real failure (a cancelled context) between the artifact insert
// and the status advance — the same atomicity property Spike 2 proved via
// real process crashes — and confirms neither effect persisted.
func TestValidateStage_FailureBetweenInsertAndAdvanceRollsBackTransaction(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	id := seedAdmitted(t, db)

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}

	if _, err := tx.ExecContext(context.Background(),
		`INSERT INTO validation_outcomes (submission_id, outcome, reason) VALUES ($1, 'valid', NULL)`,
		id,
	); err != nil {
		t.Fatalf("insert artifact: %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // force failure exactly between the insert and the status advance

	err = submission.Advance(cancelledCtx, tx, id, submission.StatusAdmitted, submission.StatusValidated)
	if err == nil {
		t.Fatal("expected the status advance to fail against a cancelled context")
	}
	_ = tx.Rollback() // mirrors validateStage's own deferred rollback

	got, getErr := submission.Get(context.Background(), db, id)
	if getErr != nil {
		t.Fatalf("get: %v", getErr)
	}
	if got.Status != submission.StatusAdmitted {
		t.Errorf("status = %q after a rolled-back transaction, want unchanged %q", got.Status, submission.StatusAdmitted)
	}
	if n := countValidationOutcomes(t, db, id); n != 0 {
		t.Errorf("validation_outcomes rows after rollback = %d, want 0", n)
	}
}
