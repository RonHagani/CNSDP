//go:build integration

package worker

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"cnsdp/internal/detection"
	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
)

// --- oldestEligibleAfter cursor semantics (Checkpoint 11): deterministic,
// no timing dependency -- these prove the structural guarantee Run's
// fairness sweep relies on, independent of any loop or poll interval.

// TestOldestEligibleAfter_NilCursor_MatchesOldestEligible proves after ==
// nil reproduces oldestEligible's original, unrestricted query exactly.
func TestOldestEligibleAfter_NilCursor_MatchesOldestEligible(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	older := seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, validEventJSON, time.Now().Add(-time.Hour))
	seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, unsupportedEventJSON, time.Now())

	sub, err := oldestEligibleAfter(context.Background(), db, nil)
	if err != nil {
		t.Fatalf("oldestEligibleAfter(nil): %v", err)
	}
	if sub.ID != older {
		t.Errorf("got submission %d, want oldest submission %d", sub.ID, older)
	}
}

// TestOldestEligibleAfter_SkipsPastCursor_ReturnsNextEligible is the
// direct proof of the sweep mechanism's building block: given a cursor at
// the oldest eligible submission's position, the next call returns the
// next-oldest eligible submission, never the one at or before the cursor.
func TestOldestEligibleAfter_SkipsPastCursor_ReturnsNextEligible(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	oldestAt := time.Now().Add(-2 * time.Hour)
	middleAt := time.Now().Add(-1 * time.Hour)
	newestAt := time.Now()

	oldest := seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, validEventJSON, oldestAt)
	middle := seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, validEventJSON, middleAt)
	newest := seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, validEventJSON, newestAt)

	got, err := oldestEligibleAfter(context.Background(), db, nil)
	if err != nil || got.ID != oldest {
		t.Fatalf("baseline: got %v, err %v, want submission %d", got, err, oldest)
	}

	got, err = oldestEligibleAfter(context.Background(), db, &cursor{oldestAt, oldest})
	if err != nil {
		t.Fatalf("after oldest cursor: %v", err)
	}
	if got.ID != middle {
		t.Errorf("after oldest cursor: got submission %d, want %d (middle)", got.ID, middle)
	}

	got, err = oldestEligibleAfter(context.Background(), db, &cursor{middleAt, middle})
	if err != nil {
		t.Fatalf("after middle cursor: %v", err)
	}
	if got.ID != newest {
		t.Errorf("after middle cursor: got submission %d, want %d (newest)", got.ID, newest)
	}

	_, err = oldestEligibleAfter(context.Background(), db, &cursor{newestAt, newest})
	if err != submission.ErrNoWork {
		t.Errorf("after newest cursor: err = %v, want ErrNoWork (nothing left in this sweep)", err)
	}
}

// --- Run: multi-submission drain and cancellation ---------------------

// seedPoisonAdmitted seeds an admitted submission together with a
// validation_outcomes row that validation.Advance's freshly computed
// Classify() result can never match -- so validation.Advance
// deterministically returns validation.ErrOutcomeConflict on every
// attempt (the exact conflict path validation.go's own doc comment
// describes), leaving the submission parked at admitted forever. This is
// a real-Postgres-driven poison submission requiring no DB mocking and no
// change to worker's *sql.DB-based signatures.
func seedPoisonAdmitted(t *testing.T, db *sql.DB, createdAt time.Time) int64 {
	t.Helper()
	id := seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, validEventJSON, createdAt)
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO validation_outcomes (submission_id, outcome, reason) VALUES ($1, 'invalid', 'poison: pre-seeded outcome deliberately mismatched for fault-isolation testing')`,
		id,
	); err != nil {
		t.Fatalf("seed poison validation outcome: %v", err)
	}
	return id
}

// logCapture is a minimal slog.Handler that records every logged
// attribute set, so tests can assert on structured fields (submission_id,
// stage, error_family) without depending on message text formatting.
type logCapture struct {
	mu      sync.Mutex
	records []map[string]string
}

func (c *logCapture) Enabled(context.Context, slog.Level) bool { return true }

func (c *logCapture) Handle(_ context.Context, r slog.Record) error {
	rec := map[string]string{"msg": r.Message}
	r.Attrs(func(a slog.Attr) bool {
		rec[a.Key] = a.Value.String()
		return true
	})
	c.mu.Lock()
	c.records = append(c.records, rec)
	c.mu.Unlock()
	return nil
}
func (c *logCapture) WithAttrs(attrs []slog.Attr) slog.Handler { return c }
func (c *logCapture) WithGroup(name string) slog.Handler       { return c }

func (c *logCapture) snapshot() []map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]map[string]string, len(c.records))
	copy(out, c.records)
	return out
}

// installLogCapture replaces the default slog logger for the duration of
// the test and restores it on cleanup.
func installLogCapture(t *testing.T) *logCapture {
	t.Helper()
	cap := &logCapture{}
	prev := slog.Default()
	slog.SetDefault(slog.New(cap))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return cap
}

func waitForStatus(t *testing.T, db *sql.DB, id int64, want submission.Status, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		sub, err := submission.Get(context.Background(), db, id)
		if err != nil {
			t.Fatalf("get submission %d: %v", id, err)
		}
		if sub.Status == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("submission %d did not reach status %q within %s (last status %q)", id, want, timeout, sub.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestRun_DrainsMultipleSubmissions_NoFailures proves the ordinary,
// no-failure path: with no stage ever failing, Run's cursor stays nil
// (never enters a sweep) and every admitted submission reaches evidenced,
// matching the fast-drain behavior repeated ProcessOne calls already
// exercise in worker_test.go.
func TestRun_DrainsMultipleSubmissions_NoFailures(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}

	var ids []int64
	base := time.Now().Add(-time.Minute)
	for i := 0; i < 4; i++ {
		ids = append(ids, seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, validEventJSON, base.Add(time.Duration(i)*time.Second)))
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); Run(ctx, db, 20*time.Millisecond) }()

	for _, id := range ids {
		waitForStatus(t, db, id, submission.StatusEvidenced, 10*time.Second)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after cancellation")
	}
}

// TestRun_ContextCancellation_ReturnsPromptly proves Run does not wait
// out a full pollInterval before returning once ctx is canceled -- the
// waitOrDone helper must observe cancellation, not just the timer.
func TestRun_ContextCancellation_ReturnsPromptly(t *testing.T) {
	db := testutil.MigratedPostgres(t) // empty: every claim attempt is ErrNoWork, exercising the wait path

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); Run(ctx, db, 2*time.Second) }()

	time.Sleep(50 * time.Millisecond) // let Run enter its poll wait
	start := time.Now()
	cancel()

	select {
	case <-done:
		if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
			t.Errorf("Run took %s to stop after cancellation, want well under the 2s poll interval", elapsed)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Run did not stop promptly after cancellation")
	}
}

// --- Run: poison-submission fairness -----------------------------------

// TestRun_PoisonSubmission_DoesNotBlockLaterHealthySubmission is the
// direct behavioral proof of the locked Checkpoint 11 fairness
// requirement: an older submission that fails every attempt must not
// prevent a newer, healthy submission from reaching evidenced, and must
// remain eligible (parked, never corrupted) rather than being dropped.
func TestRun_PoisonSubmission_DoesNotBlockLaterHealthySubmission(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}

	poison := seedPoisonAdmitted(t, db, time.Now().Add(-time.Hour)) // oldest -- would monopolize an unfixed loop
	healthy := seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, validEventJSON, time.Now())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); Run(ctx, db, 20*time.Millisecond) }()

	waitForStatus(t, db, healthy, submission.StatusEvidenced, 10*time.Second)

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after cancellation")
	}

	got, err := submission.Get(context.Background(), db, poison)
	if err != nil {
		t.Fatalf("get poison submission: %v", err)
	}
	if got.Status != submission.StatusAdmitted {
		t.Errorf("poison submission status = %q, want unchanged %q (must remain eligible, never silently dropped or corrupted)", got.Status, submission.StatusAdmitted)
	}
}

// TestRun_PoisonSubmission_AttemptedOnceThenLaterSubmissionAttemptedNext
// proves the ordering guarantee, not just the eventual outcome: the
// poison submission is the oldest eligible submission at the start of
// every sweep, so it is dispatched (and fails) first in each sweep; the
// healthy submission must then advance by exactly one stage before the
// poison submission is reattempted in the next sweep. The log sequence
// (successes are logged at debug level, failures at error level) must
// therefore strictly alternate poison-failure, healthy-success,
// poison-failure, healthy-success, ... -- two poison failures must never
// be adjacent with no healthy success between them.
func TestRun_PoisonSubmission_AttemptedOnceThenLaterSubmissionAttemptedNext(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	cap := installLogCapture(t)

	poison := seedPoisonAdmitted(t, db, time.Now().Add(-time.Hour))
	healthy := seedAtStatusWithCreatedAt(t, db, submission.StatusAdmitted, validEventJSON, time.Now())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); Run(ctx, db, 20*time.Millisecond) }()

	waitForStatus(t, db, healthy, submission.StatusEvidenced, 10*time.Second)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after cancellation")
	}

	poisonIDStr := formatID(poison)
	healthyIDStr := formatID(healthy)

	type event struct{ isPoisonFailure bool }
	var sequence []event
	for _, rec := range cap.snapshot() {
		switch {
		case rec["msg"] == "worker: stage attempt failed" && rec["submission_id"] == poisonIDStr:
			sequence = append(sequence, event{isPoisonFailure: true})
		case rec["msg"] == "worker: stage attempt succeeded" && rec["submission_id"] == healthyIDStr:
			sequence = append(sequence, event{isPoisonFailure: false})
		}
	}

	poisonFailures, healthySuccesses := 0, 0
	for _, e := range sequence {
		if e.isPoisonFailure {
			poisonFailures++
		} else {
			healthySuccesses++
		}
	}
	if poisonFailures == 0 {
		t.Fatal("expected at least one logged failure for the poison submission")
	}
	if healthySuccesses != 5 {
		t.Fatalf("expected exactly 5 logged successes for the healthy submission (one per stage admitted..alerted), got %d", healthySuccesses)
	}

	for i := 0; i < len(sequence)-1; i++ {
		if sequence[i].isPoisonFailure && sequence[i+1].isPoisonFailure {
			t.Fatalf("poison submission was reattempted (sequence position %d) with no intervening healthy-submission attempt -- fairness sweep did not yield", i)
		}
	}
	if !sequence[0].isPoisonFailure {
		t.Fatalf("expected the poison submission (older) to be attempted before the healthy submission in the first sweep")
	}
}

func formatID(id int64) string {
	b, _ := json.Marshal(id)
	return strings.Trim(string(b), `"`)
}

// TestRun_StageFailureLog_IdentifiesSubmissionStageAndFamily_NoRawContent
// proves the required log content (submission ID, stage, error family)
// and its required absence (no raw source-event content, e.g. the
// distinctive fixture username, ever appears in a logged attribute).
func TestRun_StageFailureLog_IdentifiesSubmissionStageAndFamily_NoRawContent(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	cap := installLogCapture(t)

	poison := seedPoisonAdmitted(t, db, time.Now())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); Run(ctx, db, 20*time.Millisecond) }()

	deadline := time.Now().Add(5 * time.Second)
	var found map[string]string
	for time.Now().Before(deadline) {
		for _, rec := range cap.snapshot() {
			if rec["msg"] == "worker: stage attempt failed" && rec["submission_id"] == formatID(poison) {
				found = rec
				break
			}
		}
		if found != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done

	if found == nil {
		t.Fatal("expected a logged stage-attempt-failed record for the poison submission")
	}
	if found["stage"] != string(submission.StatusAdmitted) {
		t.Errorf("stage = %q, want %q", found["stage"], submission.StatusAdmitted)
	}
	if found["error_family"] == "" {
		t.Error("error_family missing from stage-failure log")
	}
	for k, v := range found {
		if strings.Contains(v, "kubernetes-admin") { // the fixture's distinctive requesting-subject username
			t.Errorf("logged field %q = %q appears to leak raw source-event content", k, v)
		}
	}
}
