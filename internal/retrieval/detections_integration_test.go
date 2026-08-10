//go:build integration

package retrieval

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"cnsdp/internal/detection"
	"cnsdp/internal/testutil"
)

func newDetectionsTestServer(t *testing.T, db *sql.DB) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /v1/detections", &DetectionsHandler{DB: db, Token: testToken})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestDetectionsServeHTTP_ReturnsExactlyThreeActiveDefinitionsInScenarioOrder(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	srv := newDetectionsTestServer(t, db)

	resp, body := get(t, srv, "/v1/detections", testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}

	var got detectionsListResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got.Total != 3 {
		t.Fatalf("total = %d, want 3", got.Total)
	}
	if len(got.Detections) != 3 {
		t.Fatalf("len(detections) = %d, want 3", len(got.Detections))
	}

	wantScenarios := []string{"scenario-1", "scenario-2", "scenario-3"}
	for i, want := range wantScenarios {
		if got.Detections[i].Scenario != want {
			t.Errorf("detections[%d].scenario = %q, want %q", i, got.Detections[i].Scenario, want)
		}
		if got.Detections[i].Revision == "" {
			t.Errorf("detections[%d].revision is empty, want a non-empty revision id", i)
		}
		if got.Detections[i].Name == "" {
			t.Errorf("detections[%d].name is empty, want the definition's documented name", i)
		}
	}
}

// TestDetectionsServeHTTP_MatchesActiveDefinitionsContentExactly proves the
// handler is a faithful passthrough of detection.ActiveDefinitions --
// exercising the "reuse the canonical path, invent nothing" requirement
// directly, rather than only asserting hardcoded expected values.
func TestDetectionsServeHTTP_MatchesActiveDefinitionsContentExactly(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	if err := detection.Load(context.Background(), db); err != nil {
		t.Fatalf("detection.Load: %v", err)
	}
	want, err := detection.ActiveDefinitions(context.Background(), db)
	if err != nil {
		t.Fatalf("detection.ActiveDefinitions: %v", err)
	}
	srv := newDetectionsTestServer(t, db)

	resp, body := get(t, srv, "/v1/detections", testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}

	var got detectionsListResponse
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(got.Detections) != len(want) {
		t.Fatalf("len(detections) = %d, want %d", len(got.Detections), len(want))
	}
	for i, w := range want {
		g := got.Detections[i]
		if g.Scenario != w.Definition.Scenario {
			t.Errorf("detections[%d].scenario = %q, want %q", i, g.Scenario, w.Definition.Scenario)
		}
		if g.Name != w.Definition.Name {
			t.Errorf("detections[%d].name = %q, want %q", i, g.Name, w.Definition.Name)
		}
		if g.Description != w.Definition.Description {
			t.Errorf("detections[%d].description = %q, want %q", i, g.Description, w.Definition.Description)
		}
		if g.Revision != w.Revision {
			t.Errorf("detections[%d].revision = %q, want %q", i, g.Revision, w.Revision)
		}
		// Full structural comparison -- operation, requires_outcome, and
		// every requires_any/requires_all characteristic's id and
		// description -- not just slice lengths, so a handler regression
		// that drops or corrupts declared-condition content is caught here.
		if !reflect.DeepEqual(g.Conditions, w.Definition.Conditions) {
			t.Errorf("detections[%d].conditions = %+v, want %+v", i, g.Conditions, w.Definition.Conditions)
		}
	}
}

func TestDetectionsServeHTTP_MissingToken_Returns401WithNoData(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newDetectionsTestServer(t, db)

	resp, body := get(t, srv, "/v1/detections", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if len(body) != 0 {
		t.Errorf("body = %q, want empty (no data returned on denied access)", body)
	}
}

func TestDetectionsServeHTTP_InvalidToken_Returns401WithNoData(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newDetectionsTestServer(t, db)

	resp, body := get(t, srv, "/v1/detections", "wrong-token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if len(body) != 0 {
		t.Errorf("body = %q, want empty (no data returned on denied access)", body)
	}
}

func TestDetectionsServeHTTP_MethodNotAllowed_Returns405(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newDetectionsTestServer(t, db)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/detections", nil)
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
