//go:build integration

package intake

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
)

const testToken = "test-token"

func newTestServer(t *testing.T, db submission.DB) *httptest.Server {
	t.Helper()
	h := &Handler{DB: db, Token: testToken, MaxBodyBytes: DefaultMaxBodyBytes}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// newTestServerWithLimiter is newTestServer plus an explicit admission
// Limiter, for the capacity-control test cases below. Every other test in
// this file uses newTestServer (Limiter left nil), proving those
// pre-existing cases keep their exact prior behavior unchanged through the
// Handler.Limiter field's nil-safe default.
func newTestServerWithLimiter(t *testing.T, db submission.DB, limiter Limiter) *httptest.Server {
	t.Helper()
	h := &Handler{DB: db, Token: testToken, MaxBodyBytes: DefaultMaxBodyBytes, Limiter: limiter}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// makeItemsBody builds a {"items": [...]} envelope containing n distinct,
// well-formed items (unique auditIDs, so submission.Admit never treats two
// of them as the same retried submission).
func makeItemsBody(t *testing.T, n int, prefix string) string {
	t.Helper()
	items := make([]json.RawMessage, n)
	for i := 0; i < n; i++ {
		item, err := json.Marshal(map[string]string{
			"auditID": fmt.Sprintf("%s-%d", prefix, i),
			"stage":   "ResponseComplete",
		})
		if err != nil {
			t.Fatalf("marshal item %d: %v", i, err)
		}
		items[i] = item
	}
	body, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return string(body)
}

func postJSON(t *testing.T, srv *httptest.Server, body string, token string) (*http.Response, map[string]any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader([]byte(body)))
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
	var decoded map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&decoded) // best effort; some responses (401/405/etc) have no body
	return resp, decoded
}

func countSubmissions(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM submissions`).Scan(&n); err != nil {
		t.Fatalf("count submissions: %v", err)
	}
	return n
}

// failNthDB wraps a real *sql.DB and forces exactly the (0-indexed) failOn
// QueryRowContext call to fail, via a pre-cancelled context passed to the
// real connection -- the same deterministic fault-injection technique
// internal/worker's TestValidateStage_FailureBetweenInsertAndAdvanceRollsBackTransaction
// already uses against a real Postgres, not a new testing philosophy.
type failNthDB struct {
	*sql.DB
	failOn int
	calls  int
}

func (f *failNthDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	f.calls++
	if f.calls-1 == f.failOn {
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()
		return f.DB.QueryRowContext(cancelledCtx, query, args...)
	}
	return f.DB.QueryRowContext(ctx, query, args...)
}

func TestServeHTTP_MissingItems_Returns400(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, _ := postJSON(t, srv, `{}`, testToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	if n := countSubmissions(t, db); n != 0 {
		t.Errorf("submissions count = %d, want 0", n)
	}
}

func TestServeHTTP_ItemsNull_Returns400(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, _ := postJSON(t, srv, `{"items": null}`, testToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	if n := countSubmissions(t, db); n != 0 {
		t.Errorf("submissions count = %d, want 0", n)
	}
}

func TestServeHTTP_NonArrayItems_Returns400(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, _ := postJSON(t, srv, `{"items": {"a":1}}`, testToken)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
	if n := countSubmissions(t, db); n != 0 {
		t.Errorf("submissions count = %d, want 0", n)
	}
}

func TestServeHTTP_EmptyItemsArray_ReturnsEmptyResults(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, decoded := postJSON(t, srv, `{"items": []}`, testToken)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	results, _ := decoded["results"].([]any)
	if len(results) != 0 {
		t.Errorf("results = %v, want empty", results)
	}
	if n := countSubmissions(t, db); n != 0 {
		t.Errorf("submissions count = %d, want 0", n)
	}
}

func TestServeHTTP_MissingToken_Returns401(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, _ := postJSON(t, srv, `{"items": []}`, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
	if n := countSubmissions(t, db); n != 0 {
		t.Errorf("submissions count = %d, want 0", n)
	}
}

func TestServeHTTP_InvalidToken_Returns401(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	resp, _ := postJSON(t, srv, `{"items": []}`, "wrong-token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestServeHTTP_RealFixture_AdmitsWithByteExactRawEvent(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	envelope, err := os.ReadFile("testdata/scenario-1-eventlist.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var parsed struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(envelope, &parsed); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(parsed.Items) != 1 {
		t.Fatalf("fixture has %d items, want 1", len(parsed.Items))
	}
	wantRaw := []byte(parsed.Items[0])

	resp, decoded := postJSON(t, srv, string(envelope), testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	results, _ := decoded["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results = %v, want 1 entry", results)
	}
	first, _ := results[0].(map[string]any)
	idFloat, ok := first["id"].(float64)
	if !ok {
		t.Fatalf("results[0] = %v, want an id", first)
	}
	id := int64(idFloat)

	var gotRaw []byte
	var auditID, auditStage string
	if err := db.QueryRowContext(context.Background(),
		`SELECT raw_event, audit_id, audit_stage FROM submissions WHERE id = $1`, id,
	).Scan(&gotRaw, &auditID, &auditStage); err != nil {
		t.Fatalf("read submission: %v", err)
	}
	if !bytes.Equal(gotRaw, wantRaw) {
		t.Errorf("raw_event = %s, want exact fixture bytes %s", gotRaw, wantRaw)
	}
	if auditID != "34b75a57-e1c0-4659-a21f-2d39256f018c" || auditStage != "ResponseComplete" {
		t.Errorf("audit_id/audit_stage = %q/%q, want the fixture's values", auditID, auditStage)
	}
}

func TestServeHTTP_TwoItemsOneMalformed_BothAdmitted(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	body := `{"items": [{"auditID":"a1","stage":"ResponseComplete"}, "not-an-object"]}`
	resp, decoded := postJSON(t, srv, body, testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	results, _ := decoded["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("results = %v, want 2 entries", results)
	}
	for i, r := range results {
		rm, _ := r.(map[string]any)
		if _, ok := rm["id"]; !ok {
			t.Errorf("results[%d] = %v, want an id (both items should admit unconditionally)", i, rm)
		}
	}
	if n := countSubmissions(t, db); n != 2 {
		t.Errorf("submissions count = %d, want 2", n)
	}

	secondID := int64(results[1].(map[string]any)["id"].(float64))
	var auditID, auditStage string
	if err := db.QueryRowContext(context.Background(),
		`SELECT audit_id, audit_stage FROM submissions WHERE id = $1`, secondID,
	).Scan(&auditID, &auditStage); err != nil {
		t.Fatalf("read second submission: %v", err)
	}
	if auditID != "" || auditStage != "" {
		t.Errorf("malformed item's audit_id/audit_stage = %q/%q, want empty (best-effort extraction found nothing)", auditID, auditStage)
	}
}

func TestServeHTTP_RetryIdenticalItem_ReturnsSameID(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	body := `{"items": [{"auditID":"retry-1","stage":"ResponseComplete"}]}`
	resp1, decoded1 := postJSON(t, srv, body, testToken)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d, want 200", resp1.StatusCode)
	}
	resp2, decoded2 := postJSON(t, srv, body, testToken)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want 200", resp2.StatusCode)
	}

	id1 := decoded1["results"].([]any)[0].(map[string]any)["id"].(float64)
	id2 := decoded2["results"].([]any)[0].(map[string]any)["id"].(float64)
	if id1 != id2 {
		t.Errorf("retry returned id %v, want the original id %v", id2, id1)
	}
	if n := countSubmissions(t, db); n != 1 {
		t.Errorf("submissions count = %d, want 1", n)
	}
}

func TestServeHTTP_RetryIdenticalMalformedItem_ReturnsSameID(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	body := `{"items": ["not-an-object"]}`
	resp1, decoded1 := postJSON(t, srv, body, testToken)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d, want 200", resp1.StatusCode)
	}
	resp2, decoded2 := postJSON(t, srv, body, testToken)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want 200", resp2.StatusCode)
	}

	id1 := decoded1["results"].([]any)[0].(map[string]any)["id"].(float64)
	id2 := decoded2["results"].([]any)[0].(map[string]any)["id"].(float64)
	if id1 != id2 {
		t.Errorf("retry (raw-hash fallback) returned id %v, want the original id %v", id2, id1)
	}
	if n := countSubmissions(t, db); n != 1 {
		t.Errorf("submissions count = %d, want 1", n)
	}
}

func TestServeHTTP_SameIdentityDifferentContent_Returns409AndPreservesOriginal(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newTestServer(t, db)

	first := `{"items": [{"auditID":"conflict-1","stage":"ResponseComplete","a":1,"b":2}]}`
	resp1, decoded1 := postJSON(t, srv, first, testToken)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d, want 200", resp1.StatusCode)
	}
	id1 := decoded1["results"].([]any)[0].(map[string]any)["id"].(float64)

	// Same identity (auditID+stage), semantically-different raw bytes.
	second := `{"items": [{"auditID":"conflict-1","stage":"ResponseComplete","a":99,"b":100}]}`
	resp2, decoded2 := postJSON(t, srv, second, testToken)
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("second status = %d, want 409", resp2.StatusCode)
	}
	results2, _ := decoded2["results"].([]any)
	r0, _ := results2[0].(map[string]any)
	if _, hasErr := r0["error"]; !hasErr {
		t.Errorf("results[0] = %v, want an error field for the conflict", r0)
	}

	if n := countSubmissions(t, db); n != 1 {
		t.Errorf("submissions count = %d, want 1 (no duplicate/second row)", n)
	}

	var gotRaw []byte
	if err := db.QueryRowContext(context.Background(),
		`SELECT raw_event FROM submissions WHERE id = $1`, int64(id1),
	).Scan(&gotRaw); err != nil {
		t.Fatalf("read submission: %v", err)
	}
	var gotVal map[string]any
	if err := json.Unmarshal(gotRaw, &gotVal); err != nil {
		t.Fatalf("stored raw_event is not valid JSON: %v", err)
	}
	if gotVal["a"] != float64(1) || gotVal["b"] != float64(2) {
		t.Errorf("raw_event after rejected conflict = %v, want unchanged original content (a=1,b=2)", gotVal)
	}
}

// TestServeHTTP_RetryAfterPartialFailure_CompletesWithoutDuplicates covers
// both required cases together, since the retry phase needs the first
// phase's persisted state: a partial database failure produces 503 (with
// per-item results retained for diagnostics), and retrying the identical
// batch afterward completes it without creating duplicates.
func TestServeHTTP_RetryAfterPartialFailure_CompletesWithoutDuplicates(t *testing.T) {
	realDB := testutil.MigratedPostgres(t)
	failing := &failNthDB{DB: realDB, failOn: 1} // force the second item's Admit call to fail
	failingHandler := &Handler{DB: failing, Token: testToken, MaxBodyBytes: DefaultMaxBodyBytes}
	failingSrv := httptest.NewServer(failingHandler)
	defer failingSrv.Close()

	body := `{"items": [{"auditID":"partial-1","stage":"ResponseComplete"}, {"auditID":"partial-2","stage":"ResponseComplete"}]}`
	resp1, decoded1 := postJSON(t, failingSrv, body, testToken)
	if resp1.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("first status = %d, want 503", resp1.StatusCode)
	}
	results1, _ := decoded1["results"].([]any)
	if len(results1) != 2 {
		t.Fatalf("results = %v, want 2 entries", results1)
	}
	r0, _ := results1[0].(map[string]any)
	firstID, ok := r0["id"].(float64)
	if !ok {
		t.Fatalf("results[0] = %v, want an id (first item should have succeeded)", r0)
	}
	r1, _ := results1[1].(map[string]any)
	if _, ok := r1["error"]; !ok {
		t.Errorf("results[1] = %v, want an error (second item was forced to fail)", r1)
	}
	if n := countSubmissions(t, realDB); n != 1 {
		t.Fatalf("submissions count after partial failure = %d, want 1", n)
	}

	// Retry the identical batch through a normal (non-failing) handler.
	normalSrv := newTestServer(t, realDB)
	resp2, decoded2 := postJSON(t, normalSrv, body, testToken)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("retry status = %d, want 200", resp2.StatusCode)
	}
	results2, _ := decoded2["results"].([]any)
	if len(results2) != 2 {
		t.Fatalf("retry results = %v, want 2 entries", results2)
	}
	retryFirstID := results2[0].(map[string]any)["id"].(float64)
	if retryFirstID != firstID {
		t.Errorf("retry's first item id = %v, want the same id as before (%v) -- no duplicate", retryFirstID, firstID)
	}
	if _, ok := results2[1].(map[string]any)["id"]; !ok {
		t.Errorf("retry's second item = %v, want a fresh id this time", results2[1])
	}

	if n := countSubmissions(t, realDB); n != 2 {
		t.Errorf("submissions count after retry = %d, want 2 (no duplicates)", n)
	}
}

// ---- Admission-control (NFR-003, NFR-004, NFR-013; AC-022) --------------

func TestServeHTTP_ExactlyTenItemsSimultaneously_AllAdmitted(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	limiter := NewSlidingWindowLimiter(10, time.Second, nil)
	srv := newTestServerWithLimiter(t, db, limiter)

	resp, decoded := postJSON(t, srv, makeItemsBody(t, 10, "ten"), testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	results, _ := decoded["results"].([]any)
	if len(results) != 10 {
		t.Fatalf("results = %v, want 10 entries", results)
	}
	for i, r := range results {
		rm, _ := r.(map[string]any)
		if _, ok := rm["id"]; !ok {
			t.Errorf("results[%d] = %v, want an id -- 10 simultaneous items is exactly the envelope, not over it", i, rm)
		}
	}
	if n := countSubmissions(t, db); n != 10 {
		t.Errorf("submissions count = %d, want 10", n)
	}
}

func TestServeHTTP_ElevenItemsSimultaneously_TenAdmittedOneCapacityRejected(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	limiter := NewSlidingWindowLimiter(10, time.Second, nil)
	srv := newTestServerWithLimiter(t, db, limiter)

	resp, decoded := postJSON(t, srv, makeItemsBody(t, 11, "eleven"), testToken)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", resp.StatusCode)
	}
	if got := resp.Header.Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After header = %q, want %q", got, "1")
	}

	results, _ := decoded["results"].([]any)
	if len(results) != 11 {
		t.Fatalf("results = %v, want 11 entries (one per offered item, admitted or rejected)", results)
	}
	admitted, rejected := 0, 0
	for i, r := range results {
		rm, _ := r.(map[string]any)
		_, hasID := rm["id"]
		errVal, hasErr := rm["error"]
		switch {
		case hasID && !hasErr:
			admitted++
		case hasErr && !hasID:
			rejected++
			if errVal != "rejected: over capacity" {
				t.Errorf("results[%d] error = %v, want a capacity-specific message", i, errVal)
			}
		default:
			t.Errorf("results[%d] = %v, want exactly one of id/error set, never both or neither", i, rm)
		}
	}
	if admitted != 10 {
		t.Errorf("admitted count = %d, want 10", admitted)
	}
	if rejected != 1 {
		t.Errorf("capacity-rejected count = %d, want 1", rejected)
	}
	if n := countSubmissions(t, db); n != 10 {
		t.Errorf("submissions count = %d, want 10 -- the capacity-rejected item must never reach submission.Admit or be persisted", n)
	}
}

func TestServeHTTP_CapacityRefillsAcrossWindowBoundary(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	now, advance := newFakeClock(time.Unix(0, 0))
	limiter := NewSlidingWindowLimiter(10, time.Second, now)
	srv := newTestServerWithLimiter(t, db, limiter)

	resp1, _ := postJSON(t, srv, makeItemsBody(t, 10, "first"), testToken)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d, want 200", resp1.StatusCode)
	}

	resp2, _ := postJSON(t, srv, makeItemsBody(t, 1, "second"), testToken)
	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second status (capacity exhausted) = %d, want 429", resp2.StatusCode)
	}

	// Advance the injected clock exactly one window past the first
	// admission (no real sleep) -- a slot becomes available again.
	advance(time.Second)
	resp3, decoded3 := postJSON(t, srv, makeItemsBody(t, 1, "third"), testToken)
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("third status (after window advance) = %d, want 200", resp3.StatusCode)
	}
	results3, _ := decoded3["results"].([]any)
	if len(results3) != 1 {
		t.Fatalf("third results = %v, want 1 entry", results3)
	}
	if _, ok := results3[0].(map[string]any)["id"]; !ok {
		t.Errorf("third item = %v, want an id once the window has advanced", results3[0])
	}

	if n := countSubmissions(t, db); n != 11 {
		t.Errorf("submissions count = %d, want 11 (10 initial + 1 after refill; the rejected middle attempt never persisted)", n)
	}
}

func TestServeHTTP_UnauthenticatedUnderExhaustedCapacity_Returns401NotTooManyRequests(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	limiter := NewSlidingWindowLimiter(1, time.Hour, nil)
	srv := newTestServerWithLimiter(t, db, limiter)

	resp1, _ := postJSON(t, srv, makeItemsBody(t, 1, "only"), testToken)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first (authenticated) status = %d, want 200", resp1.StatusCode)
	}

	// Capacity is now fully exhausted (limit=1, window=1h). An
	// unauthenticated attempt must still be reported as unauthorized
	// (401), never as over-capacity (429): authentication is checked
	// before the admission gate is ever consulted, so an unauthenticated
	// caller cannot even observe that capacity is exhausted.
	resp2, _ := postJSON(t, srv, makeItemsBody(t, 1, "unauth"), "wrong-token")
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status under exhausted capacity = %d, want 401", resp2.StatusCode)
	}
}

func TestServeHTTP_UnauthenticatedAttempts_DoNotConsumeAdmissionCapacity(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	limiter := NewSlidingWindowLimiter(1, time.Hour, nil) // capacity for exactly one admission
	srv := newTestServerWithLimiter(t, db, limiter)

	for i := 0; i < 5; i++ {
		resp, _ := postJSON(t, srv, makeItemsBody(t, 1, "unauth"), "wrong-token")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("unauthenticated attempt %d status = %d, want 401", i, resp.StatusCode)
		}
	}

	// The single slot of real capacity must still be fully available.
	resp, decoded := postJSON(t, srv, makeItemsBody(t, 1, "auth"), testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authenticated status after 5 unauthenticated attempts = %d, want 200 -- unauthenticated attempts must never consume admission capacity", resp.StatusCode)
	}
	results, _ := decoded["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results = %v, want 1 entry", results)
	}
	if _, ok := results[0].(map[string]any)["id"]; !ok {
		t.Errorf("authenticated item = %v, want an id", results[0])
	}
	if n := countSubmissions(t, db); n != 1 {
		t.Errorf("submissions count = %d, want 1 (only the authenticated admission)", n)
	}
}

// TestServeHTTP_CapacityRejectionAndConflictInSameBatch_Returns429WithDistinguishableResults
// proves the approved status precedence (503 > 429 > 409 > 200) actually
// holds in the real handler path when a capacity rejection and a
// source-identity conflict occur in the very same batch, not merely in the
// switch statement's textual order.
func TestServeHTTP_CapacityRejectionAndConflictInSameBatch_Returns429WithDistinguishableResults(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	limiter := NewSlidingWindowLimiter(2, time.Hour, nil) // capacity for exactly two admissions
	srv := newTestServerWithLimiter(t, db, limiter)

	// Seed one submission that a later item will conflict against. Consumes
	// the first of two available capacity slots.
	seedBody := `{"items": [{"auditID":"mix-conflict-1","stage":"ResponseComplete","a":1,"b":2}]}`
	seedResp, seedDecoded := postJSON(t, srv, seedBody, testToken)
	if seedResp.StatusCode != http.StatusOK {
		t.Fatalf("seed status = %d, want 200", seedResp.StatusCode)
	}
	seedID := seedDecoded["results"].([]any)[0].(map[string]any)["id"].(float64)

	// One batch containing: (1) the same identity with different content --
	// passes the gate (consuming the second and last capacity slot), then
	// conflicts at submission.Admit -- and (2) a brand-new item arriving
	// after capacity is now fully exhausted (capacity-rejected).
	mixed := `{"items": [{"auditID":"mix-conflict-1","stage":"ResponseComplete","a":99,"b":100},{"auditID":"mix-capacity-1","stage":"ResponseComplete"}]}`
	resp, decoded := postJSON(t, srv, mixed, testToken)
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 (approved precedence 503 > 429 > 409 > 200; no failure occurred here, so capacity rejection outranks the conflict)", resp.StatusCode)
	}
	if got := resp.Header.Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want %q (final status is 429)", got, "1")
	}

	results, _ := decoded["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("results = %v, want 2 entries", results)
	}

	conflictResult, _ := results[0].(map[string]any)
	if errVal, _ := conflictResult["error"].(string); errVal != "conflict: an existing submission with different content already uses this source identity" {
		t.Errorf("results[0] error = %v, want the existing conflict message", conflictResult["error"])
	}
	if _, hasID := conflictResult["id"]; hasID {
		t.Errorf("results[0] = %v, want no id (conflict, not a fresh admission)", conflictResult)
	}

	capacityResult, _ := results[1].(map[string]any)
	if errVal, _ := capacityResult["error"].(string); errVal != "rejected: over capacity" {
		t.Errorf("results[1] error = %v, want %q", capacityResult["error"], "rejected: over capacity")
	}
	if _, hasID := capacityResult["id"]; hasID {
		t.Errorf("results[1] = %v, want no id", capacityResult)
	}

	// Only the original seed submission exists: the conflict left the
	// original row untouched (no new row created), and the
	// capacity-rejected item never reached submission.Admit at all.
	if n := countSubmissions(t, db); n != 1 {
		t.Errorf("submissions count = %d, want 1", n)
	}
	var gotRaw []byte
	if err := db.QueryRowContext(context.Background(),
		`SELECT raw_event FROM submissions WHERE id = $1`, int64(seedID),
	).Scan(&gotRaw); err != nil {
		t.Fatalf("read seed submission: %v", err)
	}
	var gotVal map[string]any
	if err := json.Unmarshal(gotRaw, &gotVal); err != nil {
		t.Fatalf("stored raw_event is not valid JSON: %v", err)
	}
	if gotVal["a"] != float64(1) || gotVal["b"] != float64(2) {
		t.Errorf("seed raw_event after conflict = %v, want unchanged original content (a=1,b=2)", gotVal)
	}
}

// TestServeHTTP_FailureAndCapacityRejectionInSameBatch_Returns503WithoutCapacityRetryAfter
// proves Retry-After is gated on the response's actual final status, not
// merely on "a capacity rejection occurred somewhere in the batch": when an
// unrelated platform fault also occurs and status ends up 503 (anyFailure
// outranks capacity rejection), the capacity-derived Retry-After must not
// be present, since it reflects the admission-control refill window
// specifically and would misleadingly suggest the failure is
// retry-after-one-second when it may not be.
func TestServeHTTP_FailureAndCapacityRejectionInSameBatch_Returns503WithoutCapacityRetryAfter(t *testing.T) {
	realDB := testutil.MigratedPostgres(t)
	failing := &failNthDB{DB: realDB, failOn: 0}          // force the first item's Admit call to fail
	limiter := NewSlidingWindowLimiter(1, time.Hour, nil) // capacity for exactly one admission
	h := &Handler{DB: failing, Token: testToken, MaxBodyBytes: DefaultMaxBodyBytes, Limiter: limiter}
	srv := httptest.NewServer(h)
	defer srv.Close()

	resp, decoded := postJSON(t, srv, makeItemsBody(t, 2, "fail-and-capacity"), testToken)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (a genuine platform fault takes precedence over capacity rejection)", resp.StatusCode)
	}
	if got := resp.Header.Get("Retry-After"); got != "" {
		t.Errorf("Retry-After = %q, want no Retry-After header -- it is capacity-specific and the final status here is 503, not 429", got)
	}

	results, _ := decoded["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("results = %v, want 2 entries", results)
	}
	r0, _ := results[0].(map[string]any)
	if errVal, _ := r0["error"].(string); errVal != "internal error" {
		t.Errorf("results[0] error = %v, want %q (the forced DB failure)", r0["error"], "internal error")
	}
	if _, hasID := r0["id"]; hasID {
		t.Errorf("results[0] = %v, want no id (the item failed, not admitted)", r0)
	}
	r1, _ := results[1].(map[string]any)
	if errVal, _ := r1["error"].(string); errVal != "rejected: over capacity" {
		t.Errorf("results[1] error = %v, want %q (the capacity-rejected item)", r1["error"], "rejected: over capacity")
	}
	if _, hasID := r1["id"]; hasID {
		t.Errorf("results[1] = %v, want no id", r1)
	}

	if n := countSubmissions(t, realDB); n != 0 {
		t.Errorf("submissions count = %d, want 0 (the failed item's insert didn't complete, and the capacity-rejected item never reached Admit)", n)
	}
}
