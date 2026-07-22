//go:build integration

package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"cnsdp/internal/detection"
	"cnsdp/internal/normalization"
	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
)

// validEventJSON is a real, complete audit.k8s.io/v1 Event (Spike 1,
// scenario 1) that classifies valid under every FR-007 item, including the
// scenario-1 completeness check (item g).
const validEventJSON = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"34b75a57-e1c0-4659-a21f-2d39256f018c","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods/high-risk-pod/exec?stdin=true&tty=true","verb":"get","user":{"username":"kubernetes-admin"},"objectRef":{"resource":"pods","namespace":"default","name":"high-risk-pod","subresource":"exec"},"responseStatus":{"code":101},"requestReceivedTimestamp":"2026-07-21T15:25:11.901891Z"}`

// unsupportedEventJSON is not attributable to the supported audit.k8s.io/v1
// Event form (FR-010) -- a stand-in for "some non-valid outcome", used to
// prove the worker's claim behavior is outcome-agnostic.
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
// with the validation_outcomes row a real validation.Advance run would
// have produced for it -- required for the outcome-aware eligibility join
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

// seedNormalizedWithCreatedAt seeds a submission at normalized status,
// with explicit created_at, together with the normalized_events row a
// real normalization.Advance run would have produced for it -- required
// for detection dispatch to have something to evaluate.
func seedNormalizedWithCreatedAt(t *testing.T, db *sql.DB, rawEvent string, createdAt time.Time) int64 {
	t.Helper()
	id := seedAtStatusWithCreatedAt(t, db, submission.StatusNormalized, rawEvent, createdAt)

	event, err := normalization.Normalize([]byte(rawEvent))
	if err != nil {
		t.Fatalf("normalize fixture: %v", err)
	}
	content, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal normalized content: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO normalized_events (submission_id, content, representation_revision) VALUES ($1, $2, $3)`,
		id, content, normalization.RepresentationRevision,
	); err != nil {
		t.Fatalf("seed normalized_events row: %v", err)
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

func countDetectionResults(t *testing.T, db *sql.DB, submissionID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM detection_results r
		 JOIN normalized_events n ON n.id = r.normalized_event_id
		 WHERE n.submission_id = $1`, submissionID,
	).Scan(&n); err != nil {
		t.Fatalf("count detection_results: %v", err)
	}
	return n
}

// --- Validate dispatch (Checkpoint 7): ProcessOne selects and dispatches
// only -- the persistence and transaction behavior of the validate stage
// itself belongs to internal/validation's own tests.

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
	if n := countValidationOutcomes(t, db, id); n != 1 {
		t.Errorf("validation_outcomes rows = %d, want 1", n)
	}
}

// TestProcessOne_ClaimsOnlyAdmitted_SkipsParkedValidatedRow is the
// worker-level proof of the locked claim-behavior decision: a submission
// already parked at validated with no recorded valid outcome (from a
// prior non-valid classification) is older but must never be reclaimed --
// reclaiming it would starve every eligible submission behind it.
func TestProcessOne_ClaimsOnlyAdmitted_SkipsParkedValidatedRow(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	parked := seedAtStatus(t, db, submission.StatusValidated, unsupportedEventJSON) // older, no outcome row, must not be touched
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
// Checkpoint 5 parked-row guarantee to the normalize dispatch: a validated
// submission whose recorded outcome is not valid (FR-014) must never be
// claimed for normalization, regardless of how long it has been waiting.
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

// --- Detection dispatch (Checkpoint 7): ProcessOne selects and dispatches
// only -- the persistence and transaction behavior of the evaluate stage
// itself belongs to internal/detection's own tests.

func TestProcessOne_AdvancesOldestNormalizedSubmission_WhenNothingElsePending(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	id := seedNormalizedWithCreatedAt(t, db, validEventJSON, time.Now())

	if err := ProcessOne(context.Background(), db); err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}

	got, err := submission.Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusEvaluated {
		t.Errorf("status = %q, want %q", got.Status, submission.StatusEvaluated)
	}
	if n := countDetectionResults(t, db, id); n != 3 {
		t.Errorf("detection_results rows = %d, want 3", n)
	}
}

// --- Cross-tier scheduling fairness (Checkpoint 6/7): eligibility
// ordering is genuinely age-based across all three claimable tiers, with
// no tier given blanket priority over another.

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

// TestProcessOne_OldestNormalized_IsNotStarvedByNewerAdmitted extends the
// no-starvation guarantee to the third tier added in Checkpoint 7: an
// older normalized submission awaiting detection evaluation must be
// claimed ahead of a newer admitted one.
func TestProcessOne_OldestNormalized_IsNotStarvedByNewerAdmitted(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	older := time.Now().Add(-1 * time.Hour)
	newer := time.Now()

	olderNormalized := seedNormalizedWithCreatedAt(t, db, validEventJSON, older)
	newerAdmitted := seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, validEventJSON, newer)

	if err := ProcessOne(context.Background(), db); err != nil {
		t.Fatalf("ProcessOne: %v", err)
	}

	gotOlder, err := submission.Get(context.Background(), db, olderNormalized)
	if err != nil {
		t.Fatalf("get older: %v", err)
	}
	if gotOlder.Status != submission.StatusEvaluated {
		t.Errorf("older normalized submission status = %q, want %q (must not be starved)", gotOlder.Status, submission.StatusEvaluated)
	}

	gotNewer, err := submission.Get(context.Background(), db, newerAdmitted)
	if err != nil {
		t.Fatalf("get newer: %v", err)
	}
	if gotNewer.Status != submission.StatusAdmitted {
		t.Errorf("newer admitted submission status = %q, want unchanged %q", gotNewer.Status, submission.StatusAdmitted)
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
