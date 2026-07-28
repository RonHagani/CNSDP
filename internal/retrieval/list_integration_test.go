//go:build integration

package retrieval

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cnsdp/internal/detection"
	"cnsdp/internal/testutil"
)

func newListTestServer(t *testing.T, db *sql.DB) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /v1/alerts", &ListHandler{DB: db, Token: testToken})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestListServeHTTP_MissingToken_Returns401(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newListTestServer(t, db)

	resp, _ := get(t, srv, "/v1/alerts", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestListServeHTTP_InvalidToken_Returns401(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newListTestServer(t, db)

	resp, _ := get(t, srv, "/v1/alerts", "wrong-token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestListServeHTTP_EmptyRepository_Returns200WithEmptyList(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newListTestServer(t, db)

	resp, body := get(t, srv, "/v1/alerts", testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}

	var got listResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Total != 0 {
		t.Errorf("total = %d, want 0", got.Total)
	}
	if len(got.Alerts) != 0 {
		t.Errorf("len(alerts) = %d, want 0", len(got.Alerts))
	}
	if !bytes.Contains(body, []byte(`"alerts":[]`)) {
		t.Errorf("expected an empty JSON array, not a null field, got: %s", body)
	}
}

func TestListServeHTTP_MultipleAlerts_ReturnsAllOrderedByIDWithSummaryFields(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	alertID1 := seedAlerted(t, db, validEventJSON, "34b75a57-e1c0-4659-a21f-2d39256f018c", "ResponseComplete")
	alertID2 := seedAlerted(t, db, validEventJSON, "45c86b68-f2d1-4760-b32f-3e4a3676129d", "ResponseComplete")
	srv := newListTestServer(t, db)

	resp, body := get(t, srv, "/v1/alerts", testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}

	var got listResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Total != 2 {
		t.Errorf("total = %d, want 2", got.Total)
	}
	if len(got.Alerts) != 2 {
		t.Fatalf("len(alerts) = %d, want 2", len(got.Alerts))
	}
	if got.Alerts[0].AlertID != alertID1 || got.Alerts[1].AlertID != alertID2 {
		t.Errorf("alert ids = [%d, %d], want [%d, %d] (ascending)", got.Alerts[0].AlertID, got.Alerts[1].AlertID, alertID1, alertID2)
	}
	row := got.Alerts[0]
	if row.DetectionName != "Interactive container exec request" {
		t.Errorf("detectionName = %q, want %q", row.DetectionName, "Interactive container exec request")
	}
	if row.Subject.Username != "kubernetes-admin" {
		t.Errorf("subject.username = %q, want kubernetes-admin", row.Subject.Username)
	}
	if row.Target.Resource != "pods" || row.Target.Name != "high-risk-pod" {
		t.Errorf("target = %+v, want resource=pods name=high-risk-pod", row.Target)
	}
	if row.Outcome.Code != 101 {
		t.Errorf("outcome.code = %v, want 101", row.Outcome.Code)
	}
	if !row.Traceability.Intact {
		t.Errorf("traceability.intact = false, want true")
	}
}

// TestListServeHTTP_NeverLeaksHeavyInvestigationPayload proves the
// dedicated-summary-contract requirement: a list response must never
// embed any row's raw event, normalized event content, or detection
// definition body -- only the lightweight summary fields a list row
// needs.
func TestListServeHTTP_NeverLeaksHeavyInvestigationPayload(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	seedAlerted(t, db, validEventJSON, "34b75a57-e1c0-4659-a21f-2d39256f018c", "ResponseComplete")
	srv := newListTestServer(t, db)

	resp, body := get(t, srv, "/v1/alerts", testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}

	// The raw audit event's distinctive auditID and requestURI fields, and
	// the full detection-definition body's condition structure, must never
	// appear in the list response -- only in GET /v1/alerts/{id}.
	forbidden := [][]byte{
		[]byte(`"rawEvent"`),
		[]byte(`"auditID"`),
		[]byte(`"definition"`),
		[]byte(`"requires_any"`),
		[]byte(`"normalizedEvent"`),
	}
	for _, f := range forbidden {
		if bytes.Contains(body, f) {
			t.Errorf("list response leaks heavy investigation payload field %s: %s", f, body)
		}
	}
}

func TestListServeHTTP_MethodNotAllowed_Returns405(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newListTestServer(t, db)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/alerts", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}
