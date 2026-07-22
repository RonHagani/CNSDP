//go:build integration

package detection

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"cnsdp/internal/normalization"
	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
)

// validEventJSON is a real, complete audit.k8s.io/v1 Event (Spike 1,
// scenario 1) -- the same fixture internal/worker and internal/validation's
// tests use. Its exec request enables both stdin and tty, so scenario-1's
// definition matches and scenario-2/3's do not.
const validEventJSON = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"34b75a57-e1c0-4659-a21f-2d39256f018c","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods/high-risk-pod/exec?stdin=true&tty=true","verb":"get","user":{"username":"kubernetes-admin"},"objectRef":{"resource":"pods","namespace":"default","name":"high-risk-pod","subresource":"exec"},"responseStatus":{"code":101},"requestReceivedTimestamp":"2026-07-21T15:25:11.901891Z"}`

// seedNormalized seeds a submission already at normalized status together
// with the normalized_events row a real normalization.Advance run would
// have produced for it -- Advance's own precondition, exercised directly
// rather than by running the full upstream pipeline first.
func seedNormalized(t *testing.T, db *sql.DB, rawEvent string) *submission.Submission {
	t.Helper()
	event, err := normalization.Normalize([]byte(rawEvent))
	if err != nil {
		t.Fatalf("normalize fixture: %v", err)
	}
	content, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal normalized content: %v", err)
	}

	digest := sha256.Sum256([]byte(rawEvent))
	var id int64
	err = db.QueryRowContext(context.Background(),
		`INSERT INTO submissions (status, raw_event, audit_id, audit_stage, source_key, raw_event_sha256)
		 VALUES ('normalized', $1, 'a', 'ResponseComplete', $2, $3) RETURNING id`,
		rawEvent, testutil.UniqueKey(t), digest[:],
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed normalized submission: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO normalized_events (submission_id, content, representation_revision) VALUES ($1, $2, $3)`,
		id, content, normalization.RepresentationRevision,
	); err != nil {
		t.Fatalf("seed normalized_events row: %v", err)
	}

	sub, err := submission.Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get seeded submission: %v", err)
	}
	return sub
}

func countDetectionResults(t *testing.T, db *sql.DB, normalizedEventID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM detection_results WHERE normalized_event_id = $1`, normalizedEventID,
	).Scan(&n); err != nil {
		t.Fatalf("count detection_results: %v", err)
	}
	return n
}

type resultRow struct {
	matched bool
	reason  sql.NullString
}

func readDetectionResultsByScenario(t *testing.T, db *sql.DB, normalizedEventID int64) map[string]resultRow {
	t.Helper()
	rows, err := db.QueryContext(context.Background(),
		`SELECT d.scenario, r.matched, r.match_reason
		 FROM detection_results r
		 JOIN detection_definitions d ON d.id = r.detection_definition_id
		 WHERE r.normalized_event_id = $1`,
		normalizedEventID,
	)
	if err != nil {
		t.Fatalf("query detection_results: %v", err)
	}
	defer rows.Close()

	out := map[string]resultRow{}
	for rows.Next() {
		var scenario string
		var rr resultRow
		if err := rows.Scan(&scenario, &rr.matched, &rr.reason); err != nil {
			t.Fatalf("scan detection_result: %v", err)
		}
		out[scenario] = rr
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate detection_results: %v", err)
	}
	return out
}

func TestAdvance_PersistsAllThreeResultsAndAdvancesStatus(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := Load(context.Background(), db); err != nil {
		t.Fatalf("Load: %v", err)
	}
	sub := seedNormalized(t, db, validEventJSON)

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusEvaluated {
		t.Errorf("status = %q, want %q", got.Status, submission.StatusEvaluated)
	}

	rec, err := normalization.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("normalization.Get: %v", err)
	}
	if n := countDetectionResults(t, db, rec.ID); n != 3 {
		t.Fatalf("detection_results rows = %d, want 3 (one per active definition)", n)
	}

	results := readDetectionResultsByScenario(t, db, rec.ID)
	s1, ok := results["scenario-1"]
	if !ok {
		t.Fatal("no detection_results row for scenario-1")
	}
	if !s1.matched {
		t.Error("scenario-1 matched = false, want true (exec request with stdin and tty)")
	}
	if !s1.reason.Valid {
		t.Error("scenario-1 match_reason = NULL, want a recorded reason for a match")
	}
	var reason MatchReason
	if err := json.Unmarshal([]byte(s1.reason.String), &reason); err != nil {
		t.Fatalf("unmarshal match_reason: %v", err)
	}
	if reason.Scenario != "scenario-1" || len(reason.SatisfiedCharacteristics) != 2 {
		t.Errorf("match reason = %+v, want scenario-1 with 2 satisfied characteristics (stdin+tty)", reason)
	}

	for _, scenario := range []string{"scenario-2", "scenario-3"} {
		r, ok := results[scenario]
		if !ok {
			t.Fatalf("no detection_results row for %s", scenario)
		}
		if r.matched {
			t.Errorf("%s matched = true, want false for a pods/exec event", scenario)
		}
		if r.reason.Valid {
			t.Errorf("%s match_reason = %q, want NULL for a non-match", scenario, r.reason.String)
		}
	}
}

func TestAdvance_RetryAfterSuccess_NoDuplicate(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := Load(context.Background(), db); err != nil {
		t.Fatalf("Load: %v", err)
	}
	sub := seedNormalized(t, db, validEventJSON)

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("first Advance: %v", err)
	}

	err := Advance(context.Background(), db, sub)
	if !errors.Is(err, submission.ErrStatusConflict) {
		t.Fatalf("second Advance: expected ErrStatusConflict, got %v", err)
	}

	rec, err := normalization.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("normalization.Get: %v", err)
	}
	if n := countDetectionResults(t, db, rec.ID); n != 3 {
		t.Errorf("detection_results rows after retry = %d, want still 3 (no duplicates)", n)
	}
	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusEvaluated {
		t.Errorf("status = %q, want unchanged %q", got.Status, submission.StatusEvaluated)
	}
}

// TestAdvance_PreexistingIdenticalResults_SafeRetrySucceeds exercises the
// race-safe insert directly: all three detection_results rows already
// exist with exactly the matched/match_reason values Advance would
// itself compute. Advance must treat this as a safe no-op on the
// artifacts and proceed to complete the guarded status advance normally.
func TestAdvance_PreexistingIdenticalResults_SafeRetrySucceeds(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := Load(context.Background(), db); err != nil {
		t.Fatalf("Load: %v", err)
	}
	sub := seedNormalized(t, db, validEventJSON)

	rec, err := normalization.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("normalization.Get: %v", err)
	}
	defs, err := ActiveDefinitions(context.Background(), db)
	if err != nil {
		t.Fatalf("ActiveDefinitions: %v", err)
	}
	for _, def := range defs {
		matched, satisfied := evaluate(rec.Event, def.Definition)
		var reason any
		if matched {
			b, err := json.Marshal(MatchReason{
				Scenario:                 def.Definition.Scenario,
				DefinitionName:           def.Definition.Name,
				DefinitionRevision:       def.Revision,
				SatisfiedCharacteristics: satisfied,
			})
			if err != nil {
				t.Fatalf("marshal reason: %v", err)
			}
			reason = b
		}
		if _, err := db.ExecContext(context.Background(),
			`INSERT INTO detection_results (normalized_event_id, detection_definition_id, matched, match_reason) VALUES ($1, $2, $3, $4)`,
			rec.ID, def.ID, matched, reason,
		); err != nil {
			t.Fatalf("pre-seed detection_results for %s: %v", def.Definition.Scenario, err)
		}
	}

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance with pre-existing identical results: %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusEvaluated {
		t.Errorf("status = %q, want %q", got.Status, submission.StatusEvaluated)
	}
	if n := countDetectionResults(t, db, rec.ID); n != 3 {
		t.Errorf("detection_results rows = %d, want 3 (no duplicates)", n)
	}
}

// TestAdvance_ConflictingExistingResult_ReturnsErrorAndLeavesNormalized
// proves ErrResultConflict: an existing detection_results row with a
// different matched value must never be silently overwritten, and the
// submission must remain at normalized rather than advance over a
// disagreement (NFR-006, NFR-017). It also proves the transaction is
// all-or-nothing across all three definitions: a conflict on one must not
// leave the other two persisted from this attempt.
func TestAdvance_ConflictingExistingResult_ReturnsErrorAndLeavesNormalized(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := Load(context.Background(), db); err != nil {
		t.Fatalf("Load: %v", err)
	}
	sub := seedNormalized(t, db, validEventJSON)

	rec, err := normalization.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("normalization.Get: %v", err)
	}
	var scenario1DefID int64
	if err := db.QueryRowContext(context.Background(),
		`SELECT id FROM detection_definitions WHERE scenario = 'scenario-1'`,
	).Scan(&scenario1DefID); err != nil {
		t.Fatalf("look up scenario-1 definition id: %v", err)
	}

	// The real outcome for scenario-1 against validEventJSON is matched=true;
	// seed the opposite to force a genuine conflict.
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO detection_results (normalized_event_id, detection_definition_id, matched, match_reason) VALUES ($1, $2, false, NULL)`,
		rec.ID, scenario1DefID,
	); err != nil {
		t.Fatalf("pre-seed conflicting detection_results row: %v", err)
	}

	err = Advance(context.Background(), db, sub)
	if !errors.Is(err, ErrResultConflict) {
		t.Fatalf("Advance: expected ErrResultConflict, got %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusNormalized {
		t.Errorf("status = %q, want unchanged %q", got.Status, submission.StatusNormalized)
	}
	if n := countDetectionResults(t, db, rec.ID); n != 1 {
		t.Errorf("detection_results rows = %d, want still 1 (conflict must abort the whole transaction, not persist the other two)", n)
	}
	results := readDetectionResultsByScenario(t, db, rec.ID)
	if s1 := results["scenario-1"]; s1.matched {
		t.Error("scenario-1 matched = true, want unchanged false (must never be overwritten)")
	}
}

// TestAdvance_CancelledContext_NoPartialEffects proves atomicity: any
// failure inside Advance's single transaction -- here, a context
// cancelled before the transaction can complete -- leaves neither any
// detection_results artifact nor the submission status changed.
func TestAdvance_CancelledContext_NoPartialEffects(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := Load(context.Background(), db); err != nil {
		t.Fatalf("Load: %v", err)
	}
	sub := seedNormalized(t, db, validEventJSON)

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := Advance(cancelledCtx, db, sub); err == nil {
		t.Fatal("expected Advance to fail against a cancelled context")
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusNormalized {
		t.Errorf("status = %q after a failed Advance, want unchanged %q", got.Status, submission.StatusNormalized)
	}

	rec, err := normalization.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("normalization.Get: %v", err)
	}
	if n := countDetectionResults(t, db, rec.ID); n != 0 {
		t.Errorf("detection_results rows = %d, want 0 (no partial effects)", n)
	}
}

// TestAdvance_MissingActiveDefinitions_ReturnsClearError proves Advance
// fails explicitly, rather than silently skipping evaluation, when the
// running application's embedded definitions were never loaded (an
// operational fault normally impossible after main.go's startup
// sequence completes).
func TestAdvance_MissingActiveDefinitions_ReturnsClearError(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	// Deliberately skip Load: detection_definitions is empty.
	sub := seedNormalized(t, db, validEventJSON)

	if err := Advance(context.Background(), db, sub); err == nil {
		t.Fatal("expected Advance to fail when active definitions are not loaded")
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusNormalized {
		t.Errorf("status = %q, want unchanged %q", got.Status, submission.StatusNormalized)
	}
}
