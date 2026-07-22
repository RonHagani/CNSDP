//go:build integration

package worker

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
)

// validEventJSON is a real, complete audit.k8s.io/v1 Event (Spike 1,
// scenario 1) that classifies valid under every FR-007 item, including the
// scenario-1 completeness check (item g).
const validEventJSON = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"34b75a57-e1c0-4659-a21f-2d39256f018c","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods/high-risk-pod/exec?stdin=true&tty=true","verb":"get","user":{"username":"kubernetes-admin"},"objectRef":{"resource":"pods","namespace":"default","name":"high-risk-pod","subresource":"exec"},"responseStatus":{"code":101},"requestReceivedTimestamp":"2026-07-21T15:25:11.901891Z"}`

// unsupportedEventJSON is not attributable to the supported audit.k8s.io/v1
// Event form (FR-010) -- a stand-in for "some non-valid outcome", used to
// prove the worker's transaction and claim behavior is outcome-agnostic.
const unsupportedEventJSON = `{"kind":"Pod","apiVersion":"v1"}`

func seedAdmitted(t *testing.T, db *sql.DB, rawEvent string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key) VALUES ($1, 'a', 'ResponseComplete', $2) RETURNING id`,
		rawEvent, testutil.UniqueKey(t),
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed admitted submission: %v", err)
	}
	return id
}

func seedAtStatus(t *testing.T, db *sql.DB, status submission.Status, rawEvent string) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO submissions (status, raw_event, audit_id, audit_stage, source_key) VALUES ($1, $2, 'a', 'ResponseComplete', $3) RETURNING id`,
		string(status), rawEvent, testutil.UniqueKey(t),
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed submission at status %s: %v", status, err)
	}
	return id
}

// seedAtStatusWithCreatedAt is seedAtStatus with an explicit created_at,
// so eligibility-ordering tests can control submission age deterministically
// instead of relying on real wall-clock gaps between INSERT statements.
func seedAtStatusWithCreatedAt(t *testing.T, db *sql.DB, status submission.Status, rawEvent string, createdAt time.Time) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO submissions (status, raw_event, audit_id, audit_stage, source_key, created_at)
		 VALUES ($1, $2, 'a', 'ResponseComplete', $3, $4) RETURNING id`,
		string(status), rawEvent, testutil.UniqueKey(t), createdAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed submission at status %s with created_at %s: %v", status, createdAt, err)
	}
	return id
}

// seedValidatedWithOutcome seeds a submission at validated status together
// with the validation_outcomes row a real validateStage run would have
// produced for it -- required for the new outcome-aware eligibility join
// (oldestEligible) to classify the row correctly.
func seedValidatedWithOutcome(t *testing.T, db *sql.DB, outcome, rawEvent string) int64 {
	t.Helper()
	id := seedAtStatus(t, db, submission.StatusValidated, rawEvent)
	var reason sql.NullString
	if outcome != "valid" {
		reason = sql.NullString{String: "test reason", Valid: true}
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO validation_outcomes (submission_id, outcome, reason) VALUES ($1, $2, $3)`,
		id, outcome, reason,
	); err != nil {
		t.Fatalf("seed validation outcome %s: %v", outcome, err)
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

func countNormalizedEvents(t *testing.T, db *sql.DB, submissionID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM normalized_events WHERE submission_id = $1`, submissionID,
	).Scan(&n); err != nil {
		t.Fatalf("count normalized_events: %v", err)
	}
	return n
}

func TestValidateStage_RecordsValidOutcomeAndAdvances(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	id := seedAdmitted(t, db, validEventJSON)

	if err := validateStage(context.Background(), db, id, []byte(validEventJSON)); err != nil {
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

// TestValidateStage_RecordsNonValidOutcomeButStillAdvances is the direct
// proof of the locked Checkpoint 5 decision: "validated" represents
// workflow progress, not a successful validation result. A non-valid
// classification still advances admitted -> validated; only the recorded
// outcome distinguishes it, and it is never reclaimed afterward (FR-014).
func TestValidateStage_RecordsNonValidOutcomeButStillAdvances(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	id := seedAdmitted(t, db, unsupportedEventJSON)

	if err := validateStage(context.Background(), db, id, []byte(unsupportedEventJSON)); err != nil {
		t.Fatalf("validateStage: %v", err)
	}

	got, err := submission.Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusValidated {
		t.Errorf("status = %q, want %q (advances regardless of outcome)", got.Status, submission.StatusValidated)
	}

	var outcome string
	var reason sql.NullString
	if err := db.QueryRowContext(context.Background(),
		`SELECT outcome, reason FROM validation_outcomes WHERE submission_id = $1`, id,
	).Scan(&outcome, &reason); err != nil {
		t.Fatalf("read validation outcome: %v", err)
	}
	if outcome != "unsupported" {
		t.Errorf("outcome = %q, want %q", outcome, "unsupported")
	}
	if !reason.Valid || reason.String == "" {
		t.Errorf("reason = %v, want a stated reason for a non-valid outcome (FR-012)", reason)
	}
}

// TestValidateStage_NoDownstreamArtifactForNonValidOutcome guards FR-014:
// a non-valid submission must never produce a normalized event.
func TestValidateStage_NoDownstreamArtifactForNonValidOutcome(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	id := seedAdmitted(t, db, unsupportedEventJSON)

	if err := validateStage(context.Background(), db, id, []byte(unsupportedEventJSON)); err != nil {
		t.Fatalf("validateStage: %v", err)
	}

	if n := countNormalizedEvents(t, db, id); n != 0 {
		t.Errorf("normalized_events rows = %d, want 0 for a non-valid outcome", n)
	}
}

func TestValidateStage_RetryAfterSuccessDoesNotDuplicate(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	id := seedAdmitted(t, db, validEventJSON)

	if err := validateStage(context.Background(), db, id, []byte(validEventJSON)); err != nil {
		t.Fatalf("first validateStage: %v", err)
	}

	// Simulate a retry against the same, now already-validated submission.
	err := validateStage(context.Background(), db, id, []byte(validEventJSON))
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
	id := seedAdmitted(t, db, validEventJSON)

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

// TestProcessOne_ClaimsOnlyAdmitted_SkipsParkedValidatedRow is the
// worker-level proof of the locked claim-behavior decision: ProcessOne must
// process only submissions currently in admitted status. A submission
// already parked at validated (from a prior non-valid classification) is
// older but must never be reclaimed -- reclaiming it would hit "no stage
// handler" every time and starve every admitted submission behind it.
func TestProcessOne_ClaimsOnlyAdmitted_SkipsParkedValidatedRow(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	parked := seedAtStatus(t, db, submission.StatusValidated, unsupportedEventJSON) // older, must not be touched
	admitted := seedAdmitted(t, db, validEventJSON)

	if err := ProcessOne(context.Background(), db); err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}

	gotAdmitted, err := submission.Get(context.Background(), db, admitted)
	if err != nil {
		t.Fatalf("get admitted: %v", err)
	}
	if gotAdmitted.Status != submission.StatusValidated {
		t.Errorf("admitted submission status = %q, want %q", gotAdmitted.Status, submission.StatusValidated)
	}

	gotParked, err := submission.Get(context.Background(), db, parked)
	if err != nil {
		t.Fatalf("get parked: %v", err)
	}
	if gotParked.Status != submission.StatusValidated {
		t.Errorf("parked submission status changed to %q, want unchanged %q", gotParked.Status, submission.StatusValidated)
	}
}

// TestProcessOne_NoWorkWhenOnlyParkedValidatedRowsExist confirms ProcessOne
// reports ErrNoWork -- not an error about a missing stage handler -- when
// every existing submission is already past admitted.
func TestProcessOne_NoWorkWhenOnlyParkedValidatedRowsExist(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	seedAtStatus(t, db, submission.StatusValidated, unsupportedEventJSON)

	err := ProcessOne(context.Background(), db)
	if !errors.Is(err, ErrNoWork) {
		t.Fatalf("expected ErrNoWork, got %v", err)
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
	id := seedAdmitted(t, db, validEventJSON)

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

// --- Normalization dispatch (Checkpoint 6): ProcessOne selects and
// dispatches only -- the persistence and transaction behavior of the
// normalize stage itself belongs to internal/normalization's own tests.

func TestProcessOne_AdvancesValidatedValidSubmission_WhenNoAdmittedPending(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	id := seedValidatedWithOutcome(t, db, "valid", validEventJSON)

	if err := ProcessOne(context.Background(), db); err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}

	got, err := submission.Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusNormalized {
		t.Errorf("status = %q, want %q", got.Status, submission.StatusNormalized)
	}
	if n := countNormalizedEvents(t, db, id); n != 1 {
		t.Errorf("normalized_events rows = %d, want 1", n)
	}
}

// TestProcessOne_SkipsValidatedNonValidOutcome_NeverNormalizes extends the
// Checkpoint 5 parked-row guarantee to the new normalize dispatch: a
// validated submission whose recorded outcome is not valid (FR-014) must
// never be claimed for normalization, regardless of how long it has been
// waiting.
func TestProcessOne_SkipsValidatedNonValidOutcome_NeverNormalizes(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	id := seedValidatedWithOutcome(t, db, "unsupported", unsupportedEventJSON)

	err := ProcessOne(context.Background(), db)
	if !errors.Is(err, ErrNoWork) {
		t.Fatalf("expected ErrNoWork, got %v", err)
	}

	got, err := submission.Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusValidated {
		t.Errorf("status = %q, want unchanged %q", got.Status, submission.StatusValidated)
	}
	if n := countNormalizedEvents(t, db, id); n != 0 {
		t.Errorf("normalized_events rows = %d, want 0", n)
	}
}

// TestProcessOne_OldestValidatedValid_IsNotStarvedByNewerAdmitted is the
// direct proof of the locked Checkpoint 6 scheduling decision: an
// "admitted always wins" policy is not approved because sustained intake
// could starve eligible validated submissions indefinitely. An older
// validated+valid submission must be claimed ahead of a newer admitted
// one.
func TestProcessOne_OldestValidatedValid_IsNotStarvedByNewerAdmitted(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	older := time.Now().Add(-1 * time.Hour)
	newer := time.Now()

	olderValidated := seedAtStatusWithCreatedAt(t, db, submission.StatusValidated, validEventJSON, older)
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO validation_outcomes (submission_id, outcome, reason) VALUES ($1, 'valid', NULL)`, olderValidated,
	); err != nil {
		t.Fatalf("seed validation outcome: %v", err)
	}
	newerAdmitted := seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, validEventJSON, newer)

	if err := ProcessOne(context.Background(), db); err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}

	gotOlder, err := submission.Get(context.Background(), db, olderValidated)
	if err != nil {
		t.Fatalf("get older: %v", err)
	}
	if gotOlder.Status != submission.StatusNormalized {
		t.Errorf("older validated+valid submission status = %q, want %q (must not be starved)", gotOlder.Status, submission.StatusNormalized)
	}

	gotNewer, err := submission.Get(context.Background(), db, newerAdmitted)
	if err != nil {
		t.Fatalf("get newer: %v", err)
	}
	if gotNewer.Status != submission.StatusAdmitted {
		t.Errorf("newer admitted submission status = %q, want unchanged %q", gotNewer.Status, submission.StatusAdmitted)
	}
}

// TestProcessOne_OlderAdmitted_IsClaimedBeforeNewerValidatedValid is the
// mirror case: eligibility ordering is genuinely age-based in both
// directions, not "validated always wins" either.
func TestProcessOne_OlderAdmitted_IsClaimedBeforeNewerValidatedValid(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	older := time.Now().Add(-1 * time.Hour)
	newer := time.Now()

	olderAdmitted := seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, validEventJSON, older)
	newerValidated := seedAtStatusWithCreatedAt(t, db, submission.StatusValidated, validEventJSON, newer)
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO validation_outcomes (submission_id, outcome, reason) VALUES ($1, 'valid', NULL)`, newerValidated,
	); err != nil {
		t.Fatalf("seed validation outcome: %v", err)
	}

	if err := ProcessOne(context.Background(), db); err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}

	gotOlder, err := submission.Get(context.Background(), db, olderAdmitted)
	if err != nil {
		t.Fatalf("get older: %v", err)
	}
	if gotOlder.Status != submission.StatusValidated {
		t.Errorf("older admitted submission status = %q, want %q", gotOlder.Status, submission.StatusValidated)
	}

	gotNewer, err := submission.Get(context.Background(), db, newerValidated)
	if err != nil {
		t.Fatalf("get newer: %v", err)
	}
	if gotNewer.Status != submission.StatusValidated {
		t.Errorf("newer validated submission status = %q, want unchanged %q", gotNewer.Status, submission.StatusValidated)
	}
}

// TestProcessOne_TieBreaksByIDWhenCreatedAtMatches proves the
// deterministic id tie-breaker for submissions created in the same
// instant.
func TestProcessOne_TieBreaksByIDWhenCreatedAtMatches(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	same := time.Now()
	first := seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, validEventJSON, same)
	second := seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, unsupportedEventJSON, same)

	if err := ProcessOne(context.Background(), db); err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}

	gotFirst, err := submission.Get(context.Background(), db, first)
	if err != nil {
		t.Fatalf("get first: %v", err)
	}
	if gotFirst.Status != submission.StatusValidated {
		t.Errorf("lower-id submission status = %q, want %q (id tie-break must prefer it)", gotFirst.Status, submission.StatusValidated)
	}

	gotSecond, err := submission.Get(context.Background(), db, second)
	if err != nil {
		t.Fatalf("get second: %v", err)
	}
	if gotSecond.Status != submission.StatusAdmitted {
		t.Errorf("higher-id submission status = %q, want unchanged %q", gotSecond.Status, submission.StatusAdmitted)
	}
}
