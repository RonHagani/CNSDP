//go:build integration

package intake

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"sync"
	"testing"

	"cnsdp/internal/testutil"
)

// resourceExhaustedNthDB wraps a real *sql.DB and forces exactly the
// (0-indexed) failOn QueryRowContext call to raise a genuine PostgreSQL
// SQLSTATE 53100 (disk_full) error -- the same deterministic
// fault-injection idiom failNthDB (intake_integration_test.go) already
// uses for a context-cancellation fault, applied here to a different
// injected condition. RAISE EXCEPTION ... USING ERRCODE against a real
// server is the same mechanism internal/db's empirical spike used to
// classify the real exhaustion condition -- this proves the product code
// path, not a mocked substitute for it.
type resourceExhaustedNthDB struct {
	*sql.DB
	failOn int
	calls  int
}

func (f *resourceExhaustedNthDB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	f.calls++
	if f.calls-1 == f.failOn {
		return f.DB.QueryRowContext(ctx, `DO $$ BEGIN RAISE EXCEPTION 'simulated disk full' USING ERRCODE = '53100'; END $$`)
	}
	return f.DB.QueryRowContext(ctx, query, args...)
}

// logCapture is a minimal slog.Handler recording every logged attribute
// set -- the same technique internal/worker's run_integration_test.go
// uses, reproduced locally since it is unexported there.
type logCapture struct {
	mu      sync.Mutex
	records []map[string]string
}

func (c *logCapture) Enabled(context.Context, slog.Level) bool { return true }

func (c *logCapture) Handle(_ context.Context, r slog.Record) error {
	rec := map[string]string{"msg": r.Message}
	r.Attrs(func(a slog.Attr) bool {
		rec[a.Key] = a.Value.String()
		return true
	})
	c.mu.Lock()
	c.records = append(c.records, rec)
	c.mu.Unlock()
	return nil
}
func (c *logCapture) WithAttrs(attrs []slog.Attr) slog.Handler { return c }
func (c *logCapture) WithGroup(name string) slog.Handler       { return c }

func (c *logCapture) snapshot() []map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]map[string]string, len(c.records))
	copy(out, c.records)
	return out
}

func installLogCapture(t *testing.T) *logCapture {
	t.Helper()
	cap := &logCapture{}
	prev := slog.Default()
	slog.SetDefault(slog.New(cap))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return cap
}

// TestServeHTTP_ResourceExhausted_Returns503WithDistinctClassification
// proves a genuine SQLSTATE 53100 admission-write failure (AC-023;
// NFR-036): reuses the existing 503 platform-fault response path (no new
// API schema), but the structured log (diagnostics.LogResourceExhausted)
// carries the distinct error_family/outcome_family =
// resource_exhausted/platform_fault classification, never the generic
// default.
func TestServeHTTP_ResourceExhausted_Returns503WithDistinctClassification(t *testing.T) {
	realDB := testutil.MigratedPostgres(t)
	failing := &resourceExhaustedNthDB{DB: realDB, failOn: 0}
	srv := newTestServer(t, failing)
	cap := installLogCapture(t)

	resp, decoded := postJSON(t, srv, `{"items": [{"auditID":"exhausted-1","stage":"ResponseComplete"}]}`, testToken)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	results, _ := decoded["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results = %v, want 1 entry", results)
	}
	r0, _ := results[0].(map[string]any)
	if errText, _ := r0["error"].(string); errText != "internal error" {
		t.Errorf(`results[0].error = %q, want "internal error" (existing response shape, unchanged)`, errText)
	}
	if _, hasID := r0["id"]; hasID {
		t.Errorf("results[0] has an id, want none -- the write did not succeed")
	}

	var sawDiagnosticsClassification bool
	for _, rec := range cap.snapshot() {
		if rec["msg"] == "diagnostics: admission write failed: resource exhausted" &&
			rec["error_family"] == "resource_exhausted" && rec["outcome_family"] == "platform_fault" {
			sawDiagnosticsClassification = true
		}
	}
	if !sawDiagnosticsClassification {
		t.Errorf("no logged record had the expected diagnostics resource_exhausted classification; records: %v", cap.snapshot())
	}
}

// TestServeHTTP_ResourceExhausted_PreviouslyAdmittedItemsUnaffected proves
// the no-partial-loss shape at the HTTP layer: an earlier item in the same
// batch that already succeeded keeps its id, and only the exhausted item
// is reported as a failure -- matching the pre-existing partial-failure
// contract this design deliberately reuses rather than replaces.
func TestServeHTTP_ResourceExhausted_PreviouslyAdmittedItemsUnaffected(t *testing.T) {
	realDB := testutil.MigratedPostgres(t)
	failing := &resourceExhaustedNthDB{DB: realDB, failOn: 1} // second item's Admit call fails
	srv := newTestServer(t, failing)

	body := `{"items": [{"auditID":"exhausted-ok-1","stage":"ResponseComplete"}, {"auditID":"exhausted-fail-1","stage":"ResponseComplete"}]}`
	resp, decoded := postJSON(t, srv, body, testToken)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	results, _ := decoded["results"].([]any)
	if len(results) != 2 {
		t.Fatalf("results = %v, want 2 entries", results)
	}
	r0, _ := results[0].(map[string]any)
	if _, ok := r0["id"].(float64); !ok {
		t.Errorf("results[0] = %v, want an id (first item should have succeeded before exhaustion hit)", r0)
	}
	r1, _ := results[1].(map[string]any)
	if _, ok := r1["error"]; !ok {
		t.Errorf("results[1] = %v, want an error (second item was forced to hit exhaustion)", r1)
	}

	var n int
	if err := realDB.QueryRow(`SELECT count(*) FROM submissions`).Scan(&n); err != nil {
		t.Fatalf("count submissions: %v", err)
	}
	if n != 1 {
		t.Errorf("submissions count = %d, want 1 (only the item before exhaustion persisted)", n)
	}
}
