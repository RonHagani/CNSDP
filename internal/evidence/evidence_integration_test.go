//go:build integration

package evidence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"testing"

	"cnsdp/internal/alerting"
	"cnsdp/internal/detection"
	"cnsdp/internal/normalization"
	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
	"cnsdp/internal/validation"
)

// validEventJSON is a real, complete audit.k8s.io/v1 Event (Spike 1,
// scenario 1) -- the same fixture used across internal/worker,
// internal/validation, internal/detection, and internal/alerting's tests.
// Its exec request enables both stdin and tty, so scenario-1's definition
// matches and scenario-2/3's do not.
const validEventJSON = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"34b75a57-e1c0-4659-a21f-2d39256f018c","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods/high-risk-pod/exec?stdin=true&tty=true","verb":"get","user":{"username":"kubernetes-admin"},"objectRef":{"resource":"pods","namespace":"default","name":"high-risk-pod","subresource":"exec"},"responseStatus":{"code":101},"requestReceivedTimestamp":"2026-07-21T15:25:11.901891Z"}`

// benignPodCreateEventJSON is a valid, scenario-2-shaped pod-creation
// event whose Pod specification carries none of scenario-2's five
// documented high-risk characteristics -- a legitimate non-match
// (AC-013 branch (a)): every detection definition evaluates it and none
// matches, so it produces zero alerts.
const benignPodCreateEventJSON = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"8f2b6a5e-1111-4c2d-9a3f-0000000000aa","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods?fieldManager=kubectl-client-side-apply","verb":"create","user":{"username":"kubernetes-admin"},"objectRef":{"resource":"pods","namespace":"default","name":"benign-pod","apiVersion":"v1"},"responseStatus":{"code":201},"requestObject":{"kind":"Pod","apiVersion":"v1","metadata":{"name":"benign-pod","namespace":"default"},"spec":{"containers":[{"name":"benign-container","image":"busybox:1.36","command":["sleep","3600"]}]}},"requestReceivedTimestamp":"2026-07-21T15:23:33.525838Z"}`

// seedAlerted runs the real pipeline -- submission.Admit,
// validation.Advance, normalization.Advance, detection.Advance,
// alerting.Advance -- against rawEvent, leaving a submission at alerted
// status with a genuine validation_outcomes row (required for evidence
// assembly's validation-outcome artifact) and whatever detection_results
// and alerts a real upstream run would have produced. detection.Load must
// already have run against db before calling this.
func seedAlerted(t *testing.T, db *sql.DB, rawEvent, auditID, auditStage string) *submission.Submission {
	t.Helper()
	ctx := context.Background()

	id, err := submission.Admit(ctx, db, json.RawMessage(rawEvent), auditID, auditStage)
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	sub, err := submission.Get(ctx, db, id)
	if err != nil {
		t.Fatalf("get admitted: %v", err)
	}
	if err := validation.Advance(ctx, db, sub); err != nil {
		t.Fatalf("validation.Advance: %v", err)
	}
	sub, err = submission.Get(ctx, db, id)
	if err != nil {
		t.Fatalf("get validated: %v", err)
	}
	if err := normalization.Advance(ctx, db, sub); err != nil {
		t.Fatalf("normalization.Advance: %v", err)
	}
	sub, err = submission.Get(ctx, db, id)
	if err != nil {
		t.Fatalf("get normalized: %v", err)
	}
	if err := detection.Advance(ctx, db, sub); err != nil {
		t.Fatalf("detection.Advance: %v", err)
	}
	sub, err = submission.Get(ctx, db, id)
	if err != nil {
		t.Fatalf("get evaluated: %v", err)
	}
	if err := alerting.Advance(ctx, db, sub); err != nil {
		t.Fatalf("alerting.Advance: %v", err)
	}
	sub, err = submission.Get(ctx, db, id)
	if err != nil {
		t.Fatalf("get alerted: %v", err)
	}
	return sub
}

// TestAssembleInventory_MatchedAlert_IncludesValidationOutcome is the
// locked Checkpoint 9 proof that the validation outcome is present in a
// complete matched-alert inventory -- not read directly from
// validation_outcomes, but through validation.Get.
func TestAssembleInventory_MatchedAlert_IncludesValidationOutcome(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	sub := seedAlerted(t, db, validEventJSON, "34b75a57-e1c0-4659-a21f-2d39256f018c", "ResponseComplete")

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

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	defer tx.Rollback()

	inv, err := assembleInventory(context.Background(), tx, sub, rec, matches[0])
	if err != nil {
		t.Fatalf("assembleInventory: %v", err)
	}
	if !inv.Complete() {
		t.Fatalf("inventory not complete: %+v", inv)
	}
	if !inv.ValidationOutcomeAvailable {
		t.Error("ValidationOutcomeAvailable = false, want true")
	}
	if inv.ValidationOutcome.Outcome != validation.OutcomeValid {
		t.Errorf("ValidationOutcome.Outcome = %q, want %q", inv.ValidationOutcome.Outcome, validation.OutcomeValid)
	}
	if !inv.Chain.Intact {
		t.Error("Chain.Intact = false, want true")
	}
}

func TestAdvance_MatchedAlert_AdvancesToEvidenced(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	sub := seedAlerted(t, db, validEventJSON, "34b75a57-e1c0-4659-a21f-2d39256f018c", "ResponseComplete")

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusEvidenced {
		t.Errorf("status = %q, want %q", got.Status, submission.StatusEvidenced)
	}
}

// TestAdvance_ZeroAlerts_VacuousVerification_AdvancesToEvidenced proves
// the locked Checkpoint 9 decision: a submission with zero alerts has an
// empty evidence-inventory set, which is still a successfully completed
// evidence stage.
func TestAdvance_ZeroAlerts_VacuousVerification_AdvancesToEvidenced(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	sub := seedAlerted(t, db, benignPodCreateEventJSON, "8f2b6a5e-1111-4c2d-9a3f-0000000000aa", "ResponseComplete")

	rec, err := normalization.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("normalization.Get: %v", err)
	}
	matches, err := detection.MatchedResults(context.Background(), db, rec.ID)
	if err != nil {
		t.Fatalf("detection.MatchedResults: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("precondition failed: %d matches, want 0", len(matches))
	}

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusEvidenced {
		t.Errorf("status = %q, want %q (unconditional advance)", got.Status, submission.StatusEvidenced)
	}
}

// TestAdvance_RawEventTamperedWithoutDigestUpdate_RollsBackAndStaysAlerted
// proves point 6/8's tampering case end to end through Advance: raw_event
// changed directly in SQL without updating raw_event_sha256 must roll
// back the whole transaction and leave the submission at alerted.
func TestAdvance_RawEventTamperedWithoutDigestUpdate_RollsBackAndStaysAlerted(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	sub := seedAlerted(t, db, validEventJSON, "34b75a57-e1c0-4659-a21f-2d39256f018c", "ResponseComplete")

	if _, err := db.ExecContext(context.Background(),
		`UPDATE submissions SET raw_event = $1 WHERE id = $2`,
		[]byte(`{"tampered":true}`), sub.ID,
	); err != nil {
		t.Fatalf("tamper raw_event: %v", err)
	}

	err := Advance(context.Background(), db, sub)
	if !errors.Is(err, ErrInventoryIncomplete) {
		t.Fatalf("Advance: expected ErrInventoryIncomplete, got %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusAlerted {
		t.Errorf("status = %q, want unchanged %q", got.Status, submission.StatusAlerted)
	}
}

// TestAdvance_AuditIdentityChangedWithoutSourceKeyUpdate_RollsBackAndStaysAlerted
// proves the companion identity-drift case: audit_id changed directly in
// SQL without updating source_key must likewise roll back and leave the
// submission at alerted.
func TestAdvance_AuditIdentityChangedWithoutSourceKeyUpdate_RollsBackAndStaysAlerted(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	sub := seedAlerted(t, db, validEventJSON, "34b75a57-e1c0-4659-a21f-2d39256f018c", "ResponseComplete")

	if _, err := db.ExecContext(context.Background(),
		`UPDATE submissions SET audit_id = 'tampered-audit-id' WHERE id = $1`,
		sub.ID,
	); err != nil {
		t.Fatalf("tamper audit_id: %v", err)
	}

	err := Advance(context.Background(), db, sub)
	if !errors.Is(err, ErrInventoryIncomplete) {
		t.Fatalf("Advance: expected ErrInventoryIncomplete, got %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusAlerted {
		t.Errorf("status = %q, want unchanged %q", got.Status, submission.StatusAlerted)
	}
}

func TestAdvance_RetryAfterSuccess_ConflictSafe(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	sub := seedAlerted(t, db, validEventJSON, "34b75a57-e1c0-4659-a21f-2d39256f018c", "ResponseComplete")

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("first Advance: %v", err)
	}

	err := Advance(context.Background(), db, sub)
	if !errors.Is(err, submission.ErrStatusConflict) {
		t.Fatalf("second Advance: expected ErrStatusConflict, got %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusEvidenced {
		t.Errorf("status = %q, want unchanged %q", got.Status, submission.StatusEvidenced)
	}
}
