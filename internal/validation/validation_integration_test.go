//go:build integration

package validation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"testing"

	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
)

// validEventJSON is a real, complete audit.k8s.io/v1 Event (Spike 1,
// scenario 1) that classifies valid under every FR-007 item, including the
// scenario-1 completeness check (item g).
const validEventJSON = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"34b75a57-e1c0-4659-a21f-2d39256f018c","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods/high-risk-pod/exec?stdin=true&tty=true","verb":"get","user":{"username":"kubernetes-admin"},"objectRef":{"resource":"pods","namespace":"default","name":"high-risk-pod","subresource":"exec"},"responseStatus":{"code":101},"requestReceivedTimestamp":"2026-07-21T15:25:11.901891Z"}`

// unsupportedEventJSON is not attributable to the supported audit.k8s.io/v1
// Event form (FR-010) -- a stand-in for "some non-valid outcome", used to
// prove Advance's transaction and retry behavior is outcome-agnostic.
const unsupportedEventJSON = `{"kind":"Pod","apiVersion":"v1"}`

func seedAdmitted(t *testing.T, db *sql.DB, rawEvent string) *submission.Submission {
	t.Helper()
	digest := sha256.Sum256([]byte(rawEvent))
	var id int64
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key, raw_event_sha256) VALUES ($1, 'a', 'ResponseComplete', $2, $3) RETURNING id`,
		rawEvent, testutil.UniqueKey(t), digest[:],
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed admitted submission: %v", err)
	}
	sub, err := submission.Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get seeded submission: %v", err)
	}
	return sub
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

func readValidationOutcome(t *testing.T, db *sql.DB, submissionID int64) (outcome string, reason sql.NullString) {
	t.Helper()
	if err := db.QueryRowContext(context.Background(),
		`SELECT outcome, reason FROM validation_outcomes WHERE submission_id = $1`, submissionID,
	).Scan(&outcome, &reason); err != nil {
		t.Fatalf("read validation outcome: %v", err)
	}
	return outcome, reason
}

func TestAdvance_RecordsValidOutcomeAndAdvances(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedAdmitted(t, db, validEventJSON)

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusValidated {
		t.Errorf("status = %q, want %q", got.Status, submission.StatusValidated)
	}
	if n := countValidationOutcomes(t, db, sub.ID); n != 1 {
		t.Errorf("validation_outcomes rows = %d, want 1", n)
	}

	outcome, reason := readValidationOutcome(t, db, sub.ID)
	if outcome != "valid" {
		t.Errorf("outcome = %q, want %q", outcome, "valid")
	}
	if reason.Valid {
		t.Errorf("reason = %q, want NULL for a valid outcome", reason.String)
	}
}

// TestAdvance_RecordsNonValidOutcomeButStillAdvances is the direct proof
// of the locked Checkpoint 5 decision, now owned by internal/validation:
// "validated" represents workflow progress, not a successful validation
// result. A non-valid classification still advances admitted -> validated;
// only the recorded outcome distinguishes it, and it is never reclaimed
// afterward (FR-014).
func TestAdvance_RecordsNonValidOutcomeButStillAdvances(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedAdmitted(t, db, unsupportedEventJSON)

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusValidated {
		t.Errorf("status = %q, want %q (advances regardless of outcome)", got.Status, submission.StatusValidated)
	}

	outcome, reason := readValidationOutcome(t, db, sub.ID)
	if outcome != "unsupported" {
		t.Errorf("outcome = %q, want %q", outcome, "unsupported")
	}
	if !reason.Valid || reason.String == "" {
		t.Errorf("reason = %v, want a stated reason for a non-valid outcome (FR-012)", reason)
	}
}

func TestAdvance_RetryAfterSuccess_NoDuplicate(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedAdmitted(t, db, validEventJSON)

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("first Advance: %v", err)
	}

	// Simulate a retry against the same, now already-validated submission.
	err := Advance(context.Background(), db, sub)
	if !errors.Is(err, submission.ErrStatusConflict) {
		t.Fatalf("expected retry to fail with ErrStatusConflict, got %v", err)
	}

	if n := countValidationOutcomes(t, db, sub.ID); n != 1 {
		t.Errorf("validation_outcomes rows after retry = %d, want still 1 (no duplicate)", n)
	}
	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusValidated {
		t.Errorf("status = %q, want %q (unchanged by the rejected retry)", got.Status, submission.StatusValidated)
	}
}

// TestAdvance_PreexistingIdenticalArtifact_SafeRetrySucceeds exercises the
// race-safe insert directly: a validation_outcomes row already exists with
// exactly the outcome and reason Advance would itself produce. Advance
// must treat this as a safe no-op on the artifact and proceed to complete
// the guarded status advance normally -- not error out and not duplicate
// the row.
func TestAdvance_PreexistingIdenticalArtifact_SafeRetrySucceeds(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedAdmitted(t, db, validEventJSON)

	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO validation_outcomes (submission_id, outcome, reason) VALUES ($1, 'valid', NULL)`,
		sub.ID,
	); err != nil {
		t.Fatalf("pre-seed identical validation_outcomes row: %v", err)
	}

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance with pre-existing identical artifact: %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusValidated {
		t.Errorf("status = %q, want %q", got.Status, submission.StatusValidated)
	}
	if n := countValidationOutcomes(t, db, sub.ID); n != 1 {
		t.Errorf("validation_outcomes rows = %d, want 1 (no duplicate)", n)
	}
}

// TestAdvance_ConflictingExistingArtifact_ReturnsErrorAndLeavesAdmitted
// proves ErrOutcomeConflict: an existing validation_outcomes row with a
// different outcome or reason must never be silently overwritten, and the
// submission must remain at admitted rather than advance over a
// disagreement (NFR-006, NFR-017).
func TestAdvance_ConflictingExistingArtifact_ReturnsErrorAndLeavesAdmitted(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedAdmitted(t, db, validEventJSON) // would classify "valid"

	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO validation_outcomes (submission_id, outcome, reason) VALUES ($1, 'unsupported', 'pre-seeded conflicting reason')`,
		sub.ID,
	); err != nil {
		t.Fatalf("pre-seed conflicting validation_outcomes row: %v", err)
	}

	err := Advance(context.Background(), db, sub)
	if !errors.Is(err, ErrOutcomeConflict) {
		t.Fatalf("Advance: expected ErrOutcomeConflict, got %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusAdmitted {
		t.Errorf("status = %q, want unchanged %q", got.Status, submission.StatusAdmitted)
	}
	outcome, reason := readValidationOutcome(t, db, sub.ID)
	if outcome != "unsupported" || reason.String != "pre-seeded conflicting reason" {
		t.Errorf("outcome/reason = %q/%q, want unchanged original (must never be overwritten)", outcome, reason.String)
	}
}

// TestAdvance_NoDownstreamArtifactForNonValidOutcome guards FR-014: a
// non-valid submission must never produce a normalized event.
func TestAdvance_NoDownstreamArtifactForNonValidOutcome(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedAdmitted(t, db, unsupportedEventJSON)

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM normalized_events WHERE submission_id = $1`, sub.ID,
	).Scan(&n); err != nil {
		t.Fatalf("count normalized_events: %v", err)
	}
	if n != 0 {
		t.Errorf("normalized_events rows = %d, want 0 for a non-valid outcome", n)
	}
}

// TestAdvance_CancelledContext_NoPartialEffects proves atomicity: any
// failure inside Advance's single transaction -- here, a context
// cancelled before the transaction can complete -- leaves neither the
// validation_outcomes artifact nor the submission status changed.
func TestAdvance_CancelledContext_NoPartialEffects(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedAdmitted(t, db, validEventJSON)

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := Advance(cancelledCtx, db, sub); err == nil {
		t.Fatal("expected Advance to fail against a cancelled context")
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusAdmitted {
		t.Errorf("status = %q after a failed Advance, want unchanged %q", got.Status, submission.StatusAdmitted)
	}
	if n := countValidationOutcomes(t, db, sub.ID); n != 0 {
		t.Errorf("validation_outcomes rows = %d, want 0 (no partial effects)", n)
	}
}

// --- Get (Checkpoint 9): the sanctioned read internal/evidence uses to
// include the persisted validation outcome as one of the six
// minimum-evidence-set artifacts.

func TestGet_ReturnsPersistedOutcome(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedAdmitted(t, db, validEventJSON)

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	got, err := Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SubmissionID != sub.ID {
		t.Errorf("SubmissionID = %d, want %d", got.SubmissionID, sub.ID)
	}
	if got.Result.Outcome != OutcomeValid {
		t.Errorf("Result.Outcome = %q, want %q", got.Result.Outcome, OutcomeValid)
	}
}

func TestGet_NotFoundBeforeAdvance(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedAdmitted(t, db, validEventJSON)

	_, err := Get(context.Background(), db, sub.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get: expected ErrNotFound, got %v", err)
	}
}
