//go:build integration

package diagnostics

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cnsdp/internal/detection"
	"cnsdp/internal/testutil"
)

// TestServeHTTP_AllChecksPass_Returns200 proves the ready branch (NFR-020)
// once both readiness checks -- database reachability and a complete
// active detection-definition set -- pass.
func TestServeHTTP_AllChecksPass_Returns200(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	h := &Handler{DB: db}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ready" {
		t.Errorf("status field = %q, want %q", body.Status, "ready")
	}
	if body.Checks["database"] != "ok" || body.Checks["detection_definitions"] != "ok" {
		t.Errorf("checks = %+v, want both ok", body.Checks)
	}
}

// TestServeHTTP_DatabaseUnreachable_Returns503WithFailedCheckDatabase
// proves the not-ready branch (AC-026's "deliberately non-operational
// state") when PostgreSQL is unreachable, without exposing the
// underlying driver error in the response.
func TestServeHTTP_DatabaseUnreachable_Returns503WithFailedCheckDatabase(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}
	h := &Handler{DB: db}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body notReadyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "not_ready" {
		t.Errorf("status field = %q, want %q", body.Status, "not_ready")
	}
	if body.FailedCheck != "database" {
		t.Errorf("failed_check = %q, want %q", body.FailedCheck, "database")
	}
}

// TestServeHTTP_DetectionDefinitionsNotLoaded_Returns503WithFailedCheckDetectionDefinitions
// proves that database reachability alone is not sufficient for
// readiness: detection.Load is deliberately never called, so
// detection.ActiveDefinitions cannot resolve the active definition set.
func TestServeHTTP_DetectionDefinitionsNotLoaded_Returns503WithFailedCheckDetectionDefinitions(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	h := &Handler{DB: db}
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	var body notReadyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.FailedCheck != "detection_definitions" {
		t.Errorf("failed_check = %q, want %q", body.FailedCheck, "detection_definitions")
	}
}
