//go:build integration

package normalization

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
)

// validEventJSON is a real, complete audit.k8s.io/v1 Event (Spike 1,
// scenario 1) -- the same fixture internal/worker's tests use, so a
// validated submission seeded here classifies exactly the way the worker
// would have already classified it before dispatching to Advance.
const validEventJSON = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"34b75a57-e1c0-4659-a21f-2d39256f018c","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods/high-risk-pod/exec?stdin=true&tty=true","verb":"get","user":{"username":"kubernetes-admin"},"objectRef":{"resource":"pods","namespace":"default","name":"high-risk-pod","subresource":"exec"},"responseStatus":{"code":101},"requestReceivedTimestamp":"2026-07-21T15:25:11.901891Z"}`

func seedValidated(t *testing.T, db *sql.DB, rawEvent string) *submission.Submission {
	t.Helper()
	var id int64
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO submissions (status, raw_event, audit_id, audit_stage, source_key)
		 VALUES ('validated', $1, 'a', 'ResponseComplete', $2) RETURNING id`,
		rawEvent, testutil.UniqueKey(t),
	).Scan(&id)
	if err != nil {
		t.Fatalf("seed validated submission: %v", err)
	}
	sub, err := submission.Get(context.Background(), db, id)
	if err != nil {
		t.Fatalf("get seeded submission: %v", err)
	}
	return sub
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

func readNormalizedEvent(t *testing.T, db *sql.DB, submissionID int64) (content []byte, revision string) {
	t.Helper()
	if err := db.QueryRowContext(context.Background(),
		`SELECT content, representation_revision FROM normalized_events WHERE submission_id = $1`, submissionID,
	).Scan(&content, &revision); err != nil {
		t.Fatalf("read normalized_events: %v", err)
	}
	return content, revision
}

func TestAdvance_PersistsNormalizedEventAndAdvancesStatus(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedValidated(t, db, validEventJSON)

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusNormalized {
		t.Errorf("status = %q, want %q", got.Status, submission.StatusNormalized)
	}
	if n := countNormalizedEvents(t, db, sub.ID); n != 1 {
		t.Fatalf("normalized_events rows = %d, want 1", n)
	}

	wantEvent, err := Normalize([]byte(validEventJSON))
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	wantContent, err := json.Marshal(wantEvent)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	gotContent, gotRevision := readNormalizedEvent(t, db, sub.ID)
	if gotRevision != RepresentationRevision {
		t.Errorf("representation_revision = %q, want %q", gotRevision, RepresentationRevision)
	}
	var gotEvent, wantEventRoundTrip Event
	if err := json.Unmarshal(gotContent, &gotEvent); err != nil {
		t.Fatalf("unmarshal stored content: %v", err)
	}
	if err := json.Unmarshal(wantContent, &wantEventRoundTrip); err != nil {
		t.Fatalf("unmarshal expected content: %v", err)
	}
	gotBytes, _ := json.Marshal(gotEvent)
	wantBytes, _ := json.Marshal(wantEventRoundTrip)
	if string(gotBytes) != string(wantBytes) {
		t.Errorf("stored content = %s, want %s", gotBytes, wantBytes)
	}
}

// TestAdvance_RetryAfterSuccess_NoDuplicate is the normalization analogue
// of the validate stage's own retry-safety proof: a second Advance call
// against a submission that has already fully committed (now normalized,
// not validated) must not duplicate the artifact and must fail the
// guarded status transition rather than silently reapply it.
func TestAdvance_RetryAfterSuccess_NoDuplicate(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedValidated(t, db, validEventJSON)

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("first Advance: %v", err)
	}

	err := Advance(context.Background(), db, sub)
	if !errors.Is(err, submission.ErrStatusConflict) {
		t.Fatalf("second Advance: expected ErrStatusConflict, got %v", err)
	}

	if n := countNormalizedEvents(t, db, sub.ID); n != 1 {
		t.Errorf("normalized_events rows after retry = %d, want still 1 (no duplicate)", n)
	}
	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusNormalized {
		t.Errorf("status = %q, want unchanged %q", got.Status, submission.StatusNormalized)
	}
}

// TestAdvance_PreexistingIdenticalArtifact_SafeRetrySucceeds exercises the
// race-safe insert directly: a normalized_events row already exists with
// exactly the content and revision Advance would itself produce (as if an
// earlier attempt's insert had somehow landed while the submission was
// still, unusually, sitting at validated). Advance must treat this as a
// safe no-op on the artifact and proceed to complete the guarded status
// advance normally -- not error out and not duplicate the row.
func TestAdvance_PreexistingIdenticalArtifact_SafeRetrySucceeds(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedValidated(t, db, validEventJSON)

	event, err := Normalize([]byte(validEventJSON))
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	content, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO normalized_events (submission_id, content, representation_revision) VALUES ($1, $2, $3)`,
		sub.ID, content, RepresentationRevision,
	); err != nil {
		t.Fatalf("pre-seed identical normalized_events row: %v", err)
	}

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance with pre-existing identical artifact: %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusNormalized {
		t.Errorf("status = %q, want %q", got.Status, submission.StatusNormalized)
	}
	if n := countNormalizedEvents(t, db, sub.ID); n != 1 {
		t.Errorf("normalized_events rows = %d, want 1 (no duplicate)", n)
	}
}

// TestAdvance_ConflictingExistingArtifact_ReturnsErrorAndLeavesValidated
// proves ErrArtifactConflict: an existing normalized_events row with
// different content must never be silently overwritten, and the
// submission must remain at validated rather than advance over a
// disagreement (NFR-006, NFR-017).
func TestAdvance_ConflictingExistingArtifact_ReturnsErrorAndLeavesValidated(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedValidated(t, db, validEventJSON)

	differentContent := []byte(`{"subject":{"username":"someone-else"}}`)
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO normalized_events (submission_id, content, representation_revision) VALUES ($1, $2, $3)`,
		sub.ID, differentContent, RepresentationRevision,
	); err != nil {
		t.Fatalf("pre-seed conflicting normalized_events row: %v", err)
	}

	err := Advance(context.Background(), db, sub)
	if !errors.Is(err, ErrArtifactConflict) {
		t.Fatalf("Advance: expected ErrArtifactConflict, got %v", err)
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusValidated {
		t.Errorf("status = %q, want unchanged %q", got.Status, submission.StatusValidated)
	}

	// JSONB storage re-serializes content (e.g. inserts spacing) even when
	// left untouched, so compare semantically rather than byte-for-byte
	// (the same "JSONB semantic equality" rule Advance itself relies on).
	gotContent, gotRevision := readNormalizedEvent(t, db, sub.ID)
	var gotDecoded, wantDecoded map[string]any
	if err := json.Unmarshal(gotContent, &gotDecoded); err != nil {
		t.Fatalf("unmarshal stored content: %v", err)
	}
	if err := json.Unmarshal(differentContent, &wantDecoded); err != nil {
		t.Fatalf("unmarshal expected content: %v", err)
	}
	if !reflect.DeepEqual(gotDecoded, wantDecoded) {
		t.Errorf("stored content = %s, want unchanged original %s (must never be overwritten)", gotContent, differentContent)
	}
	if gotRevision != RepresentationRevision {
		t.Errorf("representation_revision = %q, want unchanged %q", gotRevision, RepresentationRevision)
	}
}

// TestGet_ReturnsPersistedRecord proves Get is a faithful read of what
// Advance persisted -- the access path detection (and any future
// downstream module) must use instead of querying normalized_events
// directly.
func TestGet_ReturnsPersistedRecord(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedValidated(t, db, validEventJSON)

	if err := Advance(context.Background(), db, sub); err != nil {
		t.Fatalf("Advance: %v", err)
	}

	want, err := Normalize([]byte(validEventJSON))
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}

	got, err := Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.SubmissionID != sub.ID {
		t.Errorf("SubmissionID = %d, want %d", got.SubmissionID, sub.ID)
	}
	if got.ID <= 0 {
		t.Errorf("ID = %d, want a positive normalized_events primary key", got.ID)
	}
	if !reflect.DeepEqual(got.Event, want) {
		t.Errorf("Event = %+v, want %+v", got.Event, want)
	}
}

// TestGet_NotFoundBeforeAdvance proves Get distinguishes "not yet
// normalized" from a platform fault via a dedicated sentinel, rather than
// leaking a raw sql.ErrNoRows to callers.
func TestGet_NotFoundBeforeAdvance(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedValidated(t, db, validEventJSON)

	_, err := Get(context.Background(), db, sub.ID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get: expected ErrNotFound, got %v", err)
	}
}

// TestAdvance_CancelledContext_NoPartialEffects proves atomicity: any
// failure inside Advance's single transaction -- here, a context
// cancelled before the transaction can complete -- leaves neither the
// normalized_events artifact nor the submission status changed.
func TestAdvance_CancelledContext_NoPartialEffects(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	sub := seedValidated(t, db, validEventJSON)

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := Advance(cancelledCtx, db, sub); err == nil {
		t.Fatal("expected Advance to fail against a cancelled context")
	}

	got, err := submission.Get(context.Background(), db, sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != submission.StatusValidated {
		t.Errorf("status = %q after a failed Advance, want unchanged %q", got.Status, submission.StatusValidated)
	}
	if n := countNormalizedEvents(t, db, sub.ID); n != 0 {
		t.Errorf("normalized_events rows = %d, want 0 (no partial effects)", n)
	}
}
