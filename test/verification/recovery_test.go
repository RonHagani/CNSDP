package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cnsdp/internal/submission"
	"cnsdp/internal/validation"
)

// TestRecoveryMixEvent_IncompleteCase_ReachesOutcomeIncomplete proves the
// "incomplete" case's requestReceivedTimestamp round-trips through the real
// metav1.MicroTime parser: if it didn't, internal/validation.Classify's
// json.Unmarshal would fail before missingCoreField's user.username check
// ever runs, misclassifying this case OutcomeInvalid instead of the
// intended OutcomeIncomplete (AC-019's mixed-outcome batch requires the
// genuine outcome this case's name promises).
func TestRecoveryMixEvent_IncompleteCase_ReachesOutcomeIncomplete(t *testing.T) {
	item := recoveryMixEvent("incomplete", 1)

	result := validation.Classify(item, submission.FamilyKubernetes)
	if result.Outcome != validation.OutcomeIncomplete {
		t.Fatalf("validation.Classify(recoveryMixEvent(\"incomplete\")) = %s (%s), want %s", result.Outcome, result.Reason, validation.OutcomeIncomplete)
	}
	if !strings.Contains(result.Reason, "user") {
		t.Errorf("reason = %q, want it to name the deliberately-omitted user.username field", result.Reason)
	}
}

func TestSeedOneAdmission_RetriesOn429WithRetryAfterAndEventuallySucceeds(t *testing.T) {
	var requestCount int64
	var bodies [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, body)
		n := atomic.AddInt64(&requestCount, 1)
		if n <= 2 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"results":[{"index":0,"error":"rejected: over capacity"}]}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[{"index":0,"id":42}]}`))
	}))
	defer server.Close()

	api := NewAPIClient(server.URL, "test-token")
	item := []byte(`{"kind":"Event","auditID":"seed-1"}`)

	start := time.Now()
	id, err := seedOneAdmission(context.Background(), api, item, 5*time.Second)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("seedOneAdmission returned an error, want eventual success: %v", err)
	}
	if id != 42 {
		t.Errorf("id = %d, want 42", id)
	}
	if got := atomic.LoadInt64(&requestCount); got != 3 {
		t.Errorf("request count = %d, want 3 (2 rejections + 1 success)", got)
	}
	// Two Retry-After: 1 pauses were honored, not skipped -- proves the
	// header value drove the wait rather than an immediate hammering retry.
	if elapsed < 1800*time.Millisecond {
		t.Errorf("elapsed = %s, want >= ~1.8s (two honored Retry-After: 1 waits)", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Errorf("elapsed = %s, exceeded the 5s budget", elapsed)
	}
	if len(bodies) != 3 {
		t.Fatalf("captured %d request bodies, want 3", len(bodies))
	}
	for i, b := range bodies {
		if !bytes.Contains(b, []byte("seed-1")) {
			t.Errorf("request %d body = %s, want it to still carry the original item (same submission retried, not skipped)", i, b)
		}
	}
}

func TestSeedOneAdmission_NonCapacityErrorFailsImmediately(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"results":[{"index":0,"error":"conflict: an existing submission with different content already uses this source identity"}]}`))
	}))
	defer server.Close()

	api := NewAPIClient(server.URL, "test-token")
	item := []byte(`{"kind":"Event","auditID":"seed-2"}`)

	start := time.Now()
	id, err := seedOneAdmission(context.Background(), api, item, 5*time.Second)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("seedOneAdmission returned no error for a 409 conflict, want an immediate failure")
	}
	if id != 0 {
		t.Errorf("id = %d, want 0 on failure (a rejected item must never be counted as seeded)", id)
	}
	if got := atomic.LoadInt64(&requestCount); got != 1 {
		t.Errorf("request count = %d, want 1 (non-429 responses must not be retried)", got)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("elapsed = %s, want a near-instant failure with no retry wait", elapsed)
	}
}

func TestSeedOneAdmission_RetryBudgetIsBoundedEvenWithLargeRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "100") // far larger than the test's budget below
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"results":[{"index":0,"error":"rejected: over capacity"}]}`))
	}))
	defer server.Close()

	api := NewAPIClient(server.URL, "test-token")
	item := []byte(`{"kind":"Event","auditID":"seed-3"}`)

	start := time.Now()
	id, err := seedOneAdmission(context.Background(), api, item, 300*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("seedOneAdmission returned no error against a persistently capacity-rejecting server, want a bounded failure")
	}
	if id != 0 {
		t.Errorf("id = %d, want 0 on failure", id)
	}
	if !strings.Contains(err.Error(), "retry budget") {
		t.Errorf("error = %q, want it to name the exhausted retry budget", err.Error())
	}
	// The whole point: honoring a 100s Retry-After must never mean actually
	// waiting anywhere near 100s -- the bounded budget must win.
	if elapsed > 2*time.Second {
		t.Errorf("elapsed = %s, want well under the 100s Retry-After -- the retry budget must cap the wait, not the header", elapsed)
	}
}

func TestSeedOneAdmission_PersistentRejectionWithSmallRetryAfter_StaysBounded(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&requestCount, 1)
		// No Retry-After header at all -- exercises the fallback backoff.
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"results":[{"index":0,"error":"rejected: over capacity"}]}`))
	}))
	defer server.Close()

	api := NewAPIClient(server.URL, "test-token")
	item := []byte(`{"kind":"Event","auditID":"seed-4"}`)

	start := time.Now()
	id, err := seedOneAdmission(context.Background(), api, item, 700*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("seedOneAdmission returned no error against a persistently rejecting server, want a bounded failure")
	}
	if id != 0 {
		t.Errorf("id = %d, want 0 on failure (a rejected item must never be counted as seeded)", id)
	}
	n := atomic.LoadInt64(&requestCount)
	if n < 2 || n > 5 {
		t.Errorf("request count = %d, want a small, bounded number of attempts (not 1 -- retries did happen -- and not unbounded)", n)
	}
	if elapsed > 2*time.Second {
		t.Errorf("elapsed = %s, retries accumulated past the retry budget instead of stopping at it", elapsed)
	}
}
