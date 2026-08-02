//go:build integration

package datasources

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
)

const testToken = "test-token"

const rawEventJSON = `{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":"34b75a57-e1c0-4659-a21f-2d39256f018c","stage":"ResponseComplete"}`

func newTestServer(t *testing.T, db *sql.DB) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /v1/data-sources", &Handler{DB: db, Token: testToken})
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

func TestServeHTTP_NoSubmissions_ReportsZeroCountAndNilLastEvent(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, body := get(t, srv, "/v1/data-sources", testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}

	var got listResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Total != 1 {
		t.Fatalf("total = %d, want 1", got.Total)
	}
	if len(got.DataSources) != 1 {
		t.Fatalf("len(dataSources) = %d, want 1", len(got.DataSources))
	}
	source := got.DataSources[0]
	if source.ID != "audit-events" {
		t.Errorf("id = %q, want %q", source.ID, "audit-events")
	}
	if source.EventCount != 0 {
		t.Errorf("eventCount = %d, want 0", source.EventCount)
	}
	if source.LastEventAt != nil {
		t.Errorf("lastEventAt = %v, want nil", source.LastEventAt)
	}
}

func TestServeHTTP_OneSubmission_ReportsSingleEventAndItsTimestamp(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	ctx := context.Background()

	if _, err := submission.Admit(ctx, db, json.RawMessage(rawEventJSON), "audit-only", "ResponseComplete"); err != nil {
		t.Fatalf("admit: %v", err)
	}

	var wantLast time.Time
	if err := db.QueryRowContext(ctx, `SELECT max(created_at) FROM submissions`).Scan(&wantLast); err != nil {
		t.Fatalf("query max created_at: %v", err)
	}

	srv := newTestServer(t, db)
	resp, body := get(t, srv, "/v1/data-sources", testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}

	var got listResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Total != 1 {
		t.Errorf("total = %d, want 1", got.Total)
	}
	if len(got.DataSources) != 1 {
		t.Fatalf("len(dataSources) = %d, want 1", len(got.DataSources))
	}
	source := got.DataSources[0]
	if source.ID != "audit-events" {
		t.Errorf("id = %q, want %q", source.ID, "audit-events")
	}
	if source.DisplayName != "Audit Events API" {
		t.Errorf("displayName = %q, want %q", source.DisplayName, "Audit Events API")
	}
	if source.Endpoint != "POST /v1/audit-events" {
		t.Errorf("endpoint = %q, want %q", source.Endpoint, "POST /v1/audit-events")
	}
	if source.EventCount != 1 {
		t.Errorf("eventCount = %d, want 1", source.EventCount)
	}
	if source.LastEventAt == nil {
		t.Fatal("lastEventAt = nil, want the one submission's created_at")
	}
	if !source.LastEventAt.Equal(wantLast) {
		t.Errorf("lastEventAt = %v, want %v", source.LastEventAt, wantLast)
	}
}

func TestServeHTTP_MultipleSubmissions_ReportsCountAndLatestTimestamp(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	ctx := context.Background()

	if _, err := submission.Admit(ctx, db, json.RawMessage(rawEventJSON), "audit-1", "ResponseComplete"); err != nil {
		t.Fatalf("admit 1: %v", err)
	}
	if _, err := submission.Admit(ctx, db, json.RawMessage(rawEventJSON), "audit-2", "ResponseComplete"); err != nil {
		t.Fatalf("admit 2: %v", err)
	}
	if _, err := submission.Admit(ctx, db, json.RawMessage(rawEventJSON), "audit-3", "ResponseComplete"); err != nil {
		t.Fatalf("admit 3: %v", err)
	}

	var wantLast time.Time
	if err := db.QueryRowContext(ctx, `SELECT max(created_at) FROM submissions`).Scan(&wantLast); err != nil {
		t.Fatalf("query max created_at: %v", err)
	}

	srv := newTestServer(t, db)
	resp, body := get(t, srv, "/v1/data-sources", testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}

	var got listResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(got.DataSources) != 1 {
		t.Fatalf("len(dataSources) = %d, want 1", len(got.DataSources))
	}
	source := got.DataSources[0]
	if source.EventCount != 3 {
		t.Errorf("eventCount = %d, want 3", source.EventCount)
	}
	if source.LastEventAt == nil {
		t.Fatal("lastEventAt = nil, want the latest admitted submission's created_at")
	}
	if !source.LastEventAt.Equal(wantLast) {
		t.Errorf("lastEventAt = %v, want %v", source.LastEventAt, wantLast)
	}
}

func TestServeHTTP_MissingToken_Returns401WithNoData(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, body := get(t, srv, "/v1/data-sources", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if len(body) != 0 {
		t.Errorf("body = %q, want empty (no data returned on denied access)", body)
	}
}

func TestServeHTTP_InvalidToken_Returns401WithNoData(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, body := get(t, srv, "/v1/data-sources", "wrong-token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if len(body) != 0 {
		t.Errorf("body = %q, want empty (no data returned on denied access)", body)
	}
}

func TestServeHTTP_MethodNotAllowed_Returns405(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/data-sources", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
	if bytes.Contains(body, []byte("dataSources")) {
		t.Errorf("body = %s, want no successful data-source payload for a disallowed method", body)
	}
}
