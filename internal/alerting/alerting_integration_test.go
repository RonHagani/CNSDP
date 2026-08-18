//go:build integration

package alerting

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"cnsdp/internal/detection"
	"cnsdp/internal/normalization"
	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
)

// validEventJSON is a real, complete audit.k8s.io/v1 Event (Spike 1,
// scenario 1) -- the same fixture used across internal/worker,
// internal/validation, and internal/detection's tests. Its exec request
// enables both stdin and tty, so scenario-1's definition matches and
// scenario-2/3's do not.
const validEventJSON = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"34b75a57-e1c0-4659-a21f-2d39256f018c","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods/high-risk-pod/exec?stdin=true&tty=true","verb":"get","user":{"username":"kubernetes-admin"},"objectRef":{"resource":"pods","namespace":"default","name":"high-risk-pod","subresource":"exec"},"responseStatus":{"code":101},"requestReceivedTimestamp":"2026-07-21T15:25:11.901891Z"}`

// benignPodCreateEventJSON is a valid, scenario-2-shaped pod-creation
// event (resource pods, verb create, successful outcome) whose Pod
// specification carries none of scenario-2's five documented high-risk
// characteristics -- a legitimate non-match (AC-013 branch (a)): every
// detection definition evaluates it and none matches.
const benignPodCreateEventJSON = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"8f2b6a5e-1111-4c2d-9a3f-0000000000aa","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods?fieldManager=kubectl-client-side-apply","verb":"create","user":{"username":"kubernetes-admin"},"objectRef":{"resource":"pods","namespace":"default","name":"benign-pod","apiVersion":"v1"},"responseStatus":{"code":201},"requestObject":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"benign-pod","namespace":"default"},"spec":{"containers":[{"name":"benign-container","image":"busybox:1.36","command":["sleep","3600"]}]}},"requestReceivedTimestamp":"2026-07-21T15:23:33.525838Z"}`

// seedEvaluated runs the real normalization and detection pipeline
// against rawEvent, leaving a submission at evaluated status with
// whatever detection_results a genuine upstream run would have produced
// -- Advance's own precondition, exercised through the real Checkpoint
// 6/7 stages rather than hand-seeded rows.
func seedEvaluated(t *testing.T, db *sql.DB, rawEvent string) *submission.Submission {
	t.Helper()

	event, err := normalization.Normalize([]byte(rawEvent), submission.FamilyKubernetes)
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
	if err := detection.Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("seed detection evaluation: %v", err)
	}

	sub, err = submission.Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get evaluated submission: %v", err)
	}
	return sub
}

func countAlerts(t *testing.T, db *sql.DB, normalizedEventID int64) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(),
		`SELECT count(*) FROM alerts a
		 JOIN detection_results r ON r.id = a.detection_result_id
		 WHERE r.normalized_event_id = $1`, normalizedEventID,
	).Scan(&n); err != nil {
		t.Fatalf("count alerts: %v", err)
	}
	return n
}

func readAlertSummary(t *testing.T, db *sql.DB, normalizedEventID int64) Summary {
	t.Helper()
	var raw []byte
	err := db.QueryRowContext(context.Background(),
		`SELECT a.summary FROM alerts a
		 JOIN detection_results r ON r.id = a.detection_result_id
		 WHERE r.normalized_event_id = $1`, normalizedEventID,
	).Scan(&raw)
	if err != nil {
		t.Fatalf("read alert summary: %v", err)
	}
	var s Summary
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatalf("unmarshal alert summary: %v", err)
	}
	return s
}

func TestAdvance_MatchingSubmission_ProducesOneAlertAndAdvancesStatus(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	sub := seedEvaluated(t, db, validEventJSON)

	rec, err := normalization.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("normalization.Get: %v", err)
	}

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusAlerted {
		t.Errorf("status = %q, want %q", got.Status, submission.StatusAlerted)
	}
	if n := countAlerts(t, db, rec.ID); n != 1 {
		t.Fatalf("alerts rows = %d, want 1", n)
	}

	summary := readAlertSummary(t, db, rec.ID)
	if summary.MatchReason.Scenario != "scenario-1" {
		t.Errorf("summary.MatchReason.Scenario = %q, want scenario-1", summary.MatchReason.Scenario)
	}
	if len(summary.MatchReason.SatisfiedCharacteristics) != 2 {
		t.Errorf("satisfied characteristics = %d, want 2 (stdin+tty)", len(summary.MatchReason.SatisfiedCharacteristics))
	}
	if summary.Subject != rec.Event.Subject {
		t.Errorf("summary.Subject = %+v, want %+v", summary.Subject, rec.Event.Subject)
	}
	if summary.Target != rec.Event.Target {
		t.Errorf("summary.Target = %+v, want %+v", summary.Target, rec.Event.Target)
	}
	if !summary.RequestTime.Equal(rec.Event.RequestTime) {
		t.Errorf("summary.RequestTime = %v, want %v", summary.RequestTime, rec.Event.RequestTime)
	}
}

// TestAdvance_NoMatch_ZeroAlertsButStillAdvances proves the locked
// Checkpoint 8 decision: the evaluated -> alerted transition is
// unconditional. A legitimate non-match (AC-013 branch (a)) produces zero
// alerts but the submission still leaves evaluated, because the status
// marks completed alert-generation processing, not alert existence.
func TestAdvance_NoMatch_ZeroAlertsButStillAdvances(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	sub := seedEvaluated(t, db, benignPodCreateEventJSON)

	rec, err := normalization.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("normalization.Get: %v", err)
	}
	matches, err := detection.MatchedResults(context.Background(), db, rec.ID)
	if err != nil {
		t.Fatalf("detection.MatchedResults: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("precondition failed: fixture produced %d matches, want 0", len(matches))
	}

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusAlerted {
		t.Errorf("status = %q, want %q (unconditional advance)", got.Status, submission.StatusAlerted)
	}
	if n := countAlerts(t, db, rec.ID); n != 0 {
		t.Errorf("alerts rows = %d, want 0", n)
	}
}

func TestAdvance_RetryAfterSuccess_NoDuplicate(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	sub := seedEvaluated(t, db, validEventJSON)

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
	if n := countAlerts(t, db, rec.ID); n != 1 {
		t.Errorf("alerts rows after retry = %d, want still 1 (no duplicates)", n)
	}
}

// TestAdvance_ConflictingExistingAlert_ReturnsErrorAndLeavesEvaluated
// proves ErrSummaryConflict: an existing alerts row with a different
// summary must never be silently overwritten, and the submission must
// remain at evaluated rather than advance over a disagreement (NFR-006,
// NFR-017).
func TestAdvance_ConflictingExistingAlert_ReturnsErrorAndLeavesEvaluated(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	sub := seedEvaluated(t, db, validEventJSON)

	rec, err := normalization.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("normalization.Get: %v", err)
	}
	matches, err := detection.MatchedResults(context.Background(), db, rec.ID)
	if err != nil {
		t.Fatalf("detection.MatchedResults: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("precondition failed: %d matches, want 1", len(matches))
	}

	conflicting, err := json.Marshal(Summary{MatchReason: detection.MatchReason{Scenario: "not-what-advance-would-compute"}})
	if err != nil {
		t.Fatalf("marshal conflicting summary: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO alerts (detection_result_id, summary) VALUES ($1, $2)`,
		matches[0].ID, conflicting,
	); err != nil {
		t.Fatalf("pre-seed conflicting alerts row: %v", err)
	}

	err = Advance(context.Background(), db, sub)
	if !errors.Is(err, ErrSummaryConflict) {
		t.Fatalf("Advance: expected ErrSummaryConflict, got %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusEvaluated {
		t.Errorf("status = %q, want unchanged %q", got.Status, submission.StatusEvaluated)
	}
	if n := countAlerts(t, db, rec.ID); n != 1 {
		t.Errorf("alerts rows = %d, want still 1 (conflict must not add a second row)", n)
	}
}

// --- List (alert inventory): the read path internal/evidence.ComposeList
// uses to build the alert-inventory list, deliberately separate from Get's
// single-alert-by-detection-result lookup.

func TestList_EmptyRepository_ReturnsEmptySlice(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	got, err := List(context.Background(), db)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

// TestList_MultipleAlerts_ReturnsAllOrderedByIDAscending seeds two
// separate alerted submissions (each seedEvaluated call produces its own
// submission and normalized_events row, so both evaluate and alert
// independently even though they carry the same event content) and proves
// List returns both, in ascending id order -- a stable, deterministic
// presentation order, not insertion-order-by-accident.
func TestList_MultipleAlerts_ReturnsAllOrderedByIDAscending(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	sub1 := seedEvaluated(t, db, validEventJSON)
	if err := Advance(context.Background(), db, sub1); err != nil {
		t.Fatalf("Advance sub1: %v", err)
	}

	sub2 := seedEvaluated(t, db, validEventJSON)
	if err := Advance(context.Background(), db, sub2); err != nil {
		t.Fatalf("Advance sub2: %v", err)
	}

	got, err := List(context.Background(), db)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].ID >= got[1].ID {
		t.Errorf("alert ids = [%d, %d], want strictly ascending", got[0].ID, got[1].ID)
	}
	if got[0].Summary.MatchReason.Scenario != "scenario-1" || got[1].Summary.MatchReason.Scenario != "scenario-1" {
		t.Errorf("expected both alerts to carry scenario-1's summary, got %+v and %+v", got[0].Summary, got[1].Summary)
	}
}

// TestAdvance_LaterDefinitionRevision_DoesNotAlterExistingAlert is the
// AC-014 revision-pinning proof: after an alert has been generated, a
// newer revision is loaded for the same scenario -- via an additional
// insert, never an UPDATE of the original detection_definitions row, so
// the row the existing alert's match reason names is never mutated. The
// previously generated alert must retain its original revision and
// summary unchanged.
func TestAdvance_LaterDefinitionRevision_DoesNotAlterExistingAlert(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	sub := seedEvaluated(t, db, validEventJSON)
	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	rec, err := normalization.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("normalization.Get: %v", err)
	}
	before := readAlertSummary(t, db, rec.ID)

	newContent := []byte(`{"scenario":"scenario-1","name":"Interactive exec with TTY and stdin (revised)","description":"revised","conditions":{"operation":{"resource":"pods","subresource":"exec"},"requires_any":[{"id":"tty_allocation","description":"x"}]}}`)
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO detection_definitions (scenario, revision, content) VALUES ('scenario-1', 'newer-revision-id', $1)`,
		newContent,
	); err != nil {
		t.Fatalf("insert newer scenario-1 revision: %v", err)
	}

	after := readAlertSummary(t, db, rec.ID)
	if after.MatchReason.DefinitionRevision != before.MatchReason.DefinitionRevision {
		t.Errorf("alert revision = %q after a newer revision was loaded, want unchanged %q", after.MatchReason.DefinitionRevision, before.MatchReason.DefinitionRevision)
	}
	if after.MatchReason.DefinitionRevision == "newer-revision-id" {
		t.Error("alert resolved to the newer revision, want the original persisted revision")
	}
	if !reflect.DeepEqual(after, before) {
		t.Errorf("alert summary changed after a newer definition revision was loaded: before=%+v after=%+v", before, after)
	}
}
