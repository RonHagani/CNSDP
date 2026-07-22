//go:build integration

package retrieval

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"cnsdp/internal/alerting"
	"cnsdp/internal/detection"
	"cnsdp/internal/normalization"
	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
	"cnsdp/internal/validation"
)

const testToken = "test-token"

// validEventJSON is a real, complete audit.k8s.io/v1 Event (Spike 1,
// scenario 1) -- the same fixture used across every other checkpoint's
// tests.
const validEventJSON = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"34b75a57-e1c0-4659-a21f-2d39256f018c","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods/high-risk-pod/exec?stdin=true&tty=true","verb":"get","user":{"username":"kubernetes-admin"},"objectRef":{"resource":"pods","namespace":"default","name":"high-risk-pod","subresource":"exec"},"responseStatus":{"code":101},"requestReceivedTimestamp":"2026-07-21T15:25:11.901891Z"}`

// seedAlerted runs the real pipeline -- submission.Admit,
// validation.Advance, normalization.Advance, detection.Advance,
// alerting.Advance -- against rawEvent, leaving a submission at alerted
// status with a genuine, fully-populated chain. detection.Load must
// already have run against db before calling this.
func seedAlerted(t *testing.T, db *sql.DB, rawEvent, auditID, auditStage string) int64 {
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

	var alertID int64
	if err := db.QueryRowContext(ctx,
		`SELECT a.id FROM alerts a
		 JOIN detection_results r ON r.id = a.detection_result_id
		 JOIN normalized_events n ON n.id = r.normalized_event_id
		 WHERE n.submission_id = $1`, id,
	).Scan(&alertID); err != nil {
		t.Fatalf("resolve seeded alert id: %v", err)
	}
	return alertID
}

func newTestServer(t *testing.T, db *sql.DB) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /v1/alerts/{id}", &Handler{DB: db, Token: testToken})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, srv *httptest.Server, path, token string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp, body
}

func TestServeHTTP_MissingToken_Returns401(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, _ := get(t, srv, "/v1/alerts/1", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestServeHTTP_InvalidToken_Returns401(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, _ := get(t, srv, "/v1/alerts/1", "wrong-token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestServeHTTP_MalformedID_Returns400(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, _ := get(t, srv, "/v1/alerts/not-a-number", testToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestServeHTTP_ZeroID_Returns400(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, _ := get(t, srv, "/v1/alerts/0", testToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestServeHTTP_NegativeID_Returns400(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, _ := get(t, srv, "/v1/alerts/-1", testToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestServeHTTP_NonexistentAlert_Returns404(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, _ := get(t, srv, "/v1/alerts/999999", testToken)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestServeHTTP_MatchedAlert_Returns200WithCompleteInventory(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	alertID := seedAlerted(t, db, validEventJSON, "34b75a57-e1c0-4659-a21f-2d39256f018c", "ResponseComplete")
	srv := newTestServer(t, db)

	resp, body := get(t, srv, "/v1/alerts/"+strconv.FormatInt(alertID, 10), testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	// The raw event must appear embedded as literal JSON, never base64.
	if !bytes.Contains(body, []byte(`"auditID":"34b75a57-e1c0-4659-a21f-2d39256f018c"`)) {
		t.Errorf("response does not contain the source event embedded as literal JSON: %s", body)
	}

	var got response
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.AlertID != alertID {
		t.Errorf("alertId = %d, want %d", got.AlertID, alertID)
	}
	if !got.SourceEvent.Available || !got.ValidationOutcome.Available || !got.NormalizedEvent.Available ||
		!got.DetectionDefinition.Available || !got.DetectionResult.Available || !got.Alert.Available {
		t.Errorf("expected every artifact available, got %+v", got)
	}
	if !got.Traceability.Intact {
		t.Errorf("traceability.intact = false, want true (failedLink=%q)", got.Traceability.FailedLink)
	}
	if got.ValidationOutcome.Outcome != string(validation.OutcomeValid) {
		t.Errorf("validationOutcome.outcome = %q, want %q", got.ValidationOutcome.Outcome, validation.OutcomeValid)
	}
	if got.DetectionResult.MatchReason == nil || got.DetectionResult.MatchReason.Scenario != "scenario-1" {
		t.Errorf("detectionResult.matchReason = %+v, want scenario-1", got.DetectionResult.MatchReason)
	}
}

// TestServeHTTP_TamperedChain_Returns200WithGapVisible proves the locked
// Checkpoint 10 HTTP contract: a linked artifact failing traceability
// verification is a 200 with the gap made explicit (FR-035), never an
// error status.
func TestServeHTTP_TamperedChain_Returns200WithGapVisible(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	alertID := seedAlerted(t, db, validEventJSON, "34b75a57-e1c0-4659-a21f-2d39256f018c", "ResponseComplete")

	var submissionID int64
	if err := db.QueryRowContext(context.Background(),
		`SELECT n.submission_id FROM alerts a
		 JOIN detection_results r ON r.id = a.detection_result_id
		 JOIN normalized_events n ON n.id = r.normalized_event_id
		 WHERE a.id = $1`, alertID,
	).Scan(&submissionID); err != nil {
		t.Fatalf("resolve submission id: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`UPDATE submissions SET raw_event = $1 WHERE id = $2`,
		[]byte(`{"tampered":true}`), submissionID,
	); err != nil {
		t.Fatalf("tamper raw_event: %v", err)
	}

	srv := newTestServer(t, db)
	resp, body := get(t, srv, "/v1/alerts/"+strconv.FormatInt(alertID, 10), testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}

	var got response
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Traceability.Intact {
		t.Error("traceability.intact = true after raw_event tampering, want false")
	}
	if got.Traceability.FailedLink != "raw_event_sha256" {
		t.Errorf("traceability.failedLink = %q, want raw_event_sha256", got.Traceability.FailedLink)
	}
	// The rest of the inventory must still be present -- best-effort, not
	// suppressed by the traceability failure.
	if !got.ValidationOutcome.Available || !got.NormalizedEvent.Available || !got.DetectionDefinition.Available ||
		!got.DetectionResult.Available || !got.Alert.Available {
		t.Errorf("expected the other five artifacts still available, got %+v", got)
	}
}

// TestServeHTTP_CancelledContext_Returns500ButNeverLeaksInternalDetails
// forces a genuine infrastructure-level failure (a context already
// cancelled before Compose's database reads run) by invoking ServeHTTP
// directly rather than through a real network round trip -- the same
// technique internal/intake's own tests use for a body-read fault. The
// response body must never surface the underlying error text.
func TestServeHTTP_CancelledContext_Returns500ButNeverLeaksInternalDetails(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	alertID := seedAlerted(t, db, validEventJSON, "34b75a57-e1c0-4659-a21f-2d39256f018c", "ResponseComplete")

	h := &Handler{DB: db, Token: testToken}
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	req := httptest.NewRequest(http.MethodGet, "/v1/alerts/"+strconv.FormatInt(alertID, 10), nil).WithContext(cancelledCtx)
	req.SetPathValue("id", strconv.FormatInt(alertID, 10))
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (body: %s)", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("sql")) || bytes.Contains(rec.Body.Bytes(), []byte("context canceled")) {
		t.Errorf("response body leaks internal error detail: %s", rec.Body.String())
	}
}
