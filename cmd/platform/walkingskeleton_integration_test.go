//go:build integration

// Walking-skeleton end-to-end proof (Checkpoint 11, ARCH-01 §8): an
// authenticated HTTP POST at the real intake endpoint, driven to
// evidenced entirely by the unattended worker.Run loop, retrieved back
// through the real, authenticated retrieval endpoint. It runs in-process
// against the real handlers and the real worker package -- no production
// package is introduced solely to host this test; it lives in package
// main alongside the entry point it exercises.
package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cnsdp/internal/detection"
	"cnsdp/internal/intake"
	"cnsdp/internal/retrieval"
	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
	"cnsdp/internal/worker"
)

const testToken = "walking-skeleton-test-token"

// scenario1EventJSON is a real, complete audit.k8s.io/v1 Event (Spike 1,
// scenario 1: a pods/exec request with TTY allocation) -- the exact input
// shape ARCH-01 §8 names for the walking-skeleton demonstration.
const scenario1EventJSON = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"34b75a57-e1c0-4659-a21f-2d39256f018c","stage":"ResponseComplete","requestURI":"/api/v1/namespaces/default/pods/high-risk-pod/exec?stdin=true&tty=true","verb":"get","user":{"username":"kubernetes-admin"},"objectRef":{"resource":"pods","namespace":"default","name":"high-risk-pod","subresource":"exec"},"responseStatus":{"code":101},"requestReceivedTimestamp":"2026-07-21T15:25:11.901891Z"}`

// retrievalResponse mirrors the subset of internal/retrieval's unexported
// wire response this test needs to assert on.
type retrievalResponse struct {
	AlertID     int64 `json:"alertId"`
	SourceEvent struct {
		Available bool `json:"available"`
	} `json:"sourceEvent"`
	ValidationOutcome struct {
		Available bool   `json:"available"`
		Outcome   string `json:"outcome"`
	} `json:"validationOutcome"`
	DetectionResult struct {
		Available bool `json:"available"`
	} `json:"detectionResult"`
	Alert struct {
		Available bool `json:"available"`
	} `json:"alert"`
	Traceability struct {
		Intact bool `json:"intact"`
	} `json:"traceability"`
}

// newHarness starts a real HTTP server routing exactly main's two
// endpoints against db, and starts worker.Run against the same db with a
// short poll interval so tests complete quickly. Both are stopped and
// awaited on test cleanup.
func newHarness(t *testing.T, db *sql.DB) (baseURL string) {
	t.Helper()

	mux := http.NewServeMux()
	mux.Handle("POST /v1/audit-events", &intake.Handler{DB: db, Token: testToken})
	mux.Handle("GET /v1/alerts/{id}", &retrieval.Handler{DB: db, Token: testToken})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); worker.Run(ctx, db, 20*time.Millisecond) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("worker.Run did not stop after cancellation")
		}
	})

	return srv.URL
}

func postEventList(t *testing.T, baseURL, token string, events ...string) *http.Response {
	t.Helper()
	body := fmt.Sprintf(`{"items":[%s]}`, strings.Join(events, ","))
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/audit-events", bytes.NewReader([]byte(body)))
	if err != nil {
		t.Fatalf("build intake request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post event list: %v", err)
	}
	return resp
}

func getAlert(t *testing.T, baseURL, token string, alertID int64) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/v1/alerts/%d", baseURL, alertID), nil)
	if err != nil {
		t.Fatalf("build retrieval request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("get alert: %v", err)
	}
	return resp
}

// decodeIntakeResponse extracts the admitted submission ids, in item
// order, from a successful intake response body.
func decodeIntakeResponse(t *testing.T, resp *http.Response) []int64 {
	t.Helper()
	defer resp.Body.Close()
	var body struct {
		Results []struct {
			Index int64  `json:"index"`
			ID    int64  `json:"id"`
			Error string `json:"error"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode intake response: %v", err)
	}
	ids := make([]int64, len(body.Results))
	for i, r := range body.Results {
		if r.Error != "" {
			t.Fatalf("intake result %d: unexpected error %q", i, r.Error)
		}
		ids[i] = r.ID
	}
	return ids
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

func alertIDForSubmission(t *testing.T, db *sql.DB, submissionID int64) int64 {
	t.Helper()
	var alertID int64
	err := db.QueryRowContext(context.Background(),
		`SELECT a.id FROM alerts a
		 JOIN detection_results r ON r.id = a.detection_result_id
		 JOIN normalized_events n ON n.id = r.normalized_event_id
		 WHERE n.submission_id = $1`, submissionID,
	).Scan(&alertID)
	if err != nil {
		t.Fatalf("look up alert for submission %d: %v", submissionID, err)
	}
	return alertID
}

func withAuditID(t *testing.T, raw, auditID string) string {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	m["auditID"] = auditID
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	return string(b)
}

// TestWalkingSkeleton_IntakeToEvidencedToRetrieval is the required proof
// of ARCH-01 §8's full walking skeleton running unattended in this
// process: authenticated intake POST -> admitted -> validated ->
// normalized -> evaluated -> alerted -> evidenced, entirely driven by
// worker.Run, then an authenticated retrieval GET of the resulting
// alert. It also proves both endpoints reject unauthenticated requests
// (NFR-012, AC-024).
func TestWalkingSkeleton_IntakeToEvidencedToRetrieval(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	baseURL := newHarness(t, db)

	resp := postEventList(t, baseURL, testToken, scenario1EventJSON)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("intake POST status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	ids := decodeIntakeResponse(t, resp)
	if len(ids) != 1 {
		t.Fatalf("intake admitted %d submissions, want 1", len(ids))
	}
	submissionID := ids[0]

	waitForStatus(t, db, submissionID, submission.StatusEvidenced, 10*time.Second)

	alertID := alertIDForSubmission(t, db, submissionID)

	getResp := getAlert(t, baseURL, testToken, alertID)
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("retrieval GET status = %d, want %d", getResp.StatusCode, http.StatusOK)
	}
	var got retrievalResponse
	if err := json.NewDecoder(getResp.Body).Decode(&got); err != nil {
		t.Fatalf("decode retrieval response: %v", err)
	}
	if got.AlertID != alertID {
		t.Errorf("alertId = %d, want %d", got.AlertID, alertID)
	}
	if !got.SourceEvent.Available {
		t.Error("sourceEvent.available = false, want true")
	}
	if !got.ValidationOutcome.Available || got.ValidationOutcome.Outcome != "valid" {
		t.Errorf("validationOutcome = %+v, want available with outcome \"valid\"", got.ValidationOutcome)
	}
	if !got.DetectionResult.Available {
		t.Error("detectionResult.available = false, want true")
	}
	if !got.Alert.Available {
		t.Error("alert.available = false, want true")
	}
	if !got.Traceability.Intact {
		t.Error("traceability.intact = false, want true (full alert-to-source chain)")
	}

	// NFR-012 / AC-024: both endpoints must deny unauthenticated access.
	unauthGet := getAlert(t, baseURL, "", alertID)
	unauthGet.Body.Close()
	if unauthGet.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated retrieval GET status = %d, want %d", unauthGet.StatusCode, http.StatusUnauthorized)
	}
	unauthPost := postEventList(t, baseURL, "", scenario1EventJSON)
	unauthPost.Body.Close()
	if unauthPost.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated intake POST status = %d, want %d", unauthPost.StatusCode, http.StatusUnauthorized)
	}
}

// TestWalkingSkeleton_MultipleSubmissionsDrainInOneRunInvocation proves a
// single worker.Run invocation drives more than one submission, admitted
// together in one intake POST, all the way to evidenced.
func TestWalkingSkeleton_MultipleSubmissionsDrainInOneRunInvocation(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	baseURL := newHarness(t, db)

	var events []string
	for i := 0; i < 3; i++ {
		events = append(events, withAuditID(t, scenario1EventJSON, testutil.UniqueKey(t)))
	}

	resp := postEventList(t, baseURL, testToken, events...)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("intake POST status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	ids := decodeIntakeResponse(t, resp)
	if len(ids) != 3 {
		t.Fatalf("intake admitted %d submissions, want 3", len(ids))
	}

	for _, id := range ids {
		waitForStatus(t, db, id, submission.StatusEvidenced, 10*time.Second)
	}
}

// TestWalkingSkeleton_PoisonSubmission_DoesNotBlockHealthySubmission is
// the HTTP-level counterpart of internal/worker's poison-fairness proof:
// an older submission that fails every stage attempt must not prevent a
// newer submission, admitted through the real intake endpoint, from
// reaching evidenced and being retrievable.
func TestWalkingSkeleton_PoisonSubmission_DoesNotBlockHealthySubmission(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}

	// Seed the poison submission directly (older, pre-seeded conflicting
	// validation_outcomes row -- see internal/worker's seedPoisonAdmitted
	// for the same technique): validation.Advance's freshly computed
	// outcome for it can never match, so it deterministically fails and
	// remains parked at admitted on every attempt.
	poisonRaw := withAuditID(t, scenario1EventJSON, testutil.UniqueKey(t))
	var poisonID int64
	if err := db.QueryRowContext(context.Background(),
		`INSERT INTO submissions (raw_event, audit_id, audit_stage, source_key, raw_event_sha256, created_at)
		 VALUES ($1, 'poison', 'ResponseComplete', $2, sha256($1), $3)
		 RETURNING id`,
		[]byte(poisonRaw), testutil.UniqueKey(t), time.Now().Add(-time.Hour),
	).Scan(&poisonID); err != nil {
		t.Fatalf("seed poison submission: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO validation_outcomes (submission_id, outcome, reason) VALUES ($1, 'invalid', 'poison: pre-seeded mismatched outcome')`,
		poisonID,
	); err != nil {
		t.Fatalf("seed poison validation outcome: %v", err)
	}

	baseURL := newHarness(t, db)

	resp := postEventList(t, baseURL, testToken, withAuditID(t, scenario1EventJSON, testutil.UniqueKey(t)))
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("intake POST status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	ids := decodeIntakeResponse(t, resp)
	if len(ids) != 1 {
		t.Fatalf("intake admitted %d submissions, want 1", len(ids))
	}
	healthyID := ids[0]

	waitForStatus(t, db, healthyID, submission.StatusEvidenced, 10*time.Second)

	alertID := alertIDForSubmission(t, db, healthyID)
	getResp := getAlert(t, baseURL, testToken, alertID)
	getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Errorf("retrieval GET for healthy submission's alert status = %d, want %d", getResp.StatusCode, http.StatusOK)
	}

	poison, err := submission.Get(context.Background(), db, poisonID)
	if err != nil {
		t.Fatalf("get poison submission: %v", err)
	}
	if poison.Status != submission.StatusAdmitted {
		t.Errorf("poison submission status = %q, want unchanged %q (must remain eligible, never dropped)", poison.Status, submission.StatusAdmitted)
	}
}

// TestWalkingSkeleton_HarnessCancellation_CompletesWithinDeadline proves
// the harness's worker.Run goroutine (the same primitive main's shutdown
// lifecycle relies on) stops promptly on cancellation rather than waiting
// out a full poll interval.
func TestWalkingSkeleton_HarnessCancellation_CompletesWithinDeadline(t *testing.T) {
	db := testutil.MigratedPostgres(t)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); worker.Run(ctx, db, 2*time.Second) }()

	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	cancel()

	select {
	case <-done:
		if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
			t.Errorf("worker.Run took %s to stop, want well under the 2s poll interval", elapsed)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("worker.Run did not stop within the deadline")
	}
}
