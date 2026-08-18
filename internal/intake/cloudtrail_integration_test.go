//go:build integration

package intake

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cnsdp/internal/submission"
	"cnsdp/internal/testutil"
)

func newCloudTrailTestServer(t *testing.T, db submission.DB) *httptest.Server {
	t.Helper()
	h := &CloudTrailHandler{DB: db, Token: testToken, MaxBodyBytes: DefaultMaxBodyBytes}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// newCloudTrailTestServerWithLimiter is newCloudTrailTestServer plus an
// explicit admission Limiter -- the CloudTrail-family counterpart to
// newTestServerWithLimiter, used both for this endpoint's own capacity
// tests and for the cross-endpoint shared-limiter proof below.
func newCloudTrailTestServerWithLimiter(t *testing.T, db submission.DB, limiter Limiter) *httptest.Server {
	t.Helper()
	h := &CloudTrailHandler{DB: db, Token: testToken, MaxBodyBytes: DefaultMaxBodyBytes, Limiter: limiter}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestCloudTrailServeHTTP_MissingToken_Returns401(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newCloudTrailTestServer(t, db)

	resp, _ := postJSON(t, srv, `{"eventID":"evt-noauth"}`, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if n := countSubmissions(t, db); n != 0 {
		t.Errorf("submissions count = %d, want 0 (unauthenticated attempt must not be admitted)", n)
	}
}

func TestCloudTrailServeHTTP_InvalidToken_Returns401(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newCloudTrailTestServer(t, db)

	resp, _ := postJSON(t, srv, `{"eventID":"evt-badauth"}`, "wrong-token")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

// TestCloudTrailServeHTTP_RealRecord_AdmitsWithByteExactRawEventAndSourceEventID
// is the CloudTrail analog of intake_integration_test.go's
// TestServeHTTP_RealFixture_AdmitsWithByteExactRawEvent: the exact posted
// bytes are preserved verbatim, source_family is recorded as the
// attribution signal (B.1 of the approved design -- which endpoint
// received the submission, not content-sniffing), and source_event_id
// carries the record's own eventID.
func TestCloudTrailServeHTTP_RealRecord_AdmitsWithByteExactRawEventAndSourceEventID(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newCloudTrailTestServer(t, db)

	const raw = `{"eventVersion":"1.08","eventTime":"2026-08-18T12:34:56Z","eventSource":"iam.amazonaws.com","eventName":"CreateAccessKey","eventCategory":"Management","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/alice"},"requestParameters":{"userName":"bob"},"responseElements":{"accessKey":{"userName":"bob","accessKeyId":"AKIAEXAMPLE"}},"eventID":"evt-http-1","eventType":"AwsApiCall"}`

	resp, decoded := postJSON(t, srv, raw, testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	idFloat, ok := decoded["id"].(float64)
	if !ok {
		t.Fatalf("response = %v, want an id", decoded)
	}
	id := int64(idFloat)

	var gotRaw []byte
	var family, sourceEventID, auditID, auditStage string
	if err := db.QueryRowContext(context.Background(),
		`SELECT raw_event, source_family, source_event_id, audit_id, audit_stage FROM submissions WHERE id = $1`, id,
	).Scan(&gotRaw, &family, &sourceEventID, &auditID, &auditStage); err != nil {
		t.Fatalf("read submission: %v", err)
	}
	if !bytes.Equal(gotRaw, []byte(raw)) {
		t.Errorf("raw_event = %s, want exact posted bytes %s", gotRaw, raw)
	}
	if family != string(submission.FamilyCloudTrail) {
		t.Errorf("source_family = %q, want %q", family, submission.FamilyCloudTrail)
	}
	if sourceEventID != "evt-http-1" {
		t.Errorf("source_event_id = %q, want %q", sourceEventID, "evt-http-1")
	}
	if auditID != "" || auditStage != "" {
		t.Errorf("audit_id/audit_stage = %q/%q, want both empty for a CloudTrail submission", auditID, auditStage)
	}
}

// TestCloudTrailServeHTTP_MalformedBody_StillAdmitted proves FR-001's
// admit-everything-regardless-of-content-shape principle holds at the
// CloudTrail endpoint too: there is no envelope-parsing gate the way the
// Kubernetes endpoint's {"items":[...]} wrapper has, since the entire body
// -- however malformed -- is itself the one submission. Classification
// (Unsupported, here) is validation's job, not intake's.
func TestCloudTrailServeHTTP_MalformedBody_StillAdmitted(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newCloudTrailTestServer(t, db)

	resp, decoded := postJSON(t, srv, `not json at all`, testToken)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (admitted despite malformed content)", resp.StatusCode)
	}
	if _, ok := decoded["id"]; !ok {
		t.Fatalf("response = %v, want an id", decoded)
	}
	if n := countSubmissions(t, db); n != 1 {
		t.Errorf("submissions count = %d, want 1", n)
	}
}

func TestCloudTrailServeHTTP_RetryByteIdenticalRecord_ReturnsSameID(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newCloudTrailTestServer(t, db)

	body := `{"eventID":"evt-retry-1","eventName":"CreateAccessKey"}`
	resp1, decoded1 := postJSON(t, srv, body, testToken)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d, want 200", resp1.StatusCode)
	}
	resp2, decoded2 := postJSON(t, srv, body, testToken)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("second status = %d, want 200", resp2.StatusCode)
	}

	id1 := decoded1["id"].(float64)
	id2 := decoded2["id"].(float64)
	if id1 != id2 {
		t.Errorf("retry returned id %v, want the original id %v", id2, id1)
	}
	if n := countSubmissions(t, db); n != 1 {
		t.Errorf("submissions count = %d, want 1 (byte-identical redelivery must not duplicate)", n)
	}
}

func TestCloudTrailServeHTTP_SameEventIDDifferentContent_Returns409AndPreservesOriginal(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	srv := newCloudTrailTestServer(t, db)

	first := `{"eventID":"evt-conflict-1","eventName":"CreateAccessKey"}`
	resp1, decoded1 := postJSON(t, srv, first, testToken)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d, want 200", resp1.StatusCode)
	}
	id1 := int64(decoded1["id"].(float64))

	second := `{"eventID":"evt-conflict-1","eventName":"DeactivateMFADevice"}`
	resp2, decoded2 := postJSON(t, srv, second, testToken)
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("second status = %d, want 409", resp2.StatusCode)
	}
	if _, hasID := decoded2["id"]; hasID {
		t.Errorf("conflict response = %v, want no id", decoded2)
	}

	var gotRaw []byte
	if err := db.QueryRowContext(context.Background(),
		`SELECT raw_event FROM submissions WHERE id = $1`, id1,
	).Scan(&gotRaw); err != nil {
		t.Fatalf("read submission: %v", err)
	}
	if !bytes.Equal(gotRaw, []byte(first)) {
		t.Errorf("raw_event after rejected conflict = %s, want unchanged original %s", gotRaw, first)
	}
	if n := countSubmissions(t, db); n != 1 {
		t.Errorf("submissions count = %d, want 1 (no duplicate row from the conflicting attempt)", n)
	}
}

func TestCloudTrailServeHTTP_CapacityRejection_Returns429(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	limiter := NewSlidingWindowLimiter(1, time.Second, nil)
	srv := newCloudTrailTestServerWithLimiter(t, db, limiter)

	resp1, _ := postJSON(t, srv, `{"eventID":"evt-cap-1"}`, testToken)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first status = %d, want 200", resp1.StatusCode)
	}

	resp2, decoded2 := postJSON(t, srv, `{"eventID":"evt-cap-2"}`, testToken)
	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429", resp2.StatusCode)
	}
	if got := resp2.Header.Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After header = %q, want %q", got, "1")
	}
	if decoded2["error"] != "rejected: over capacity" {
		t.Errorf("error = %v, want a capacity-specific message", decoded2["error"])
	}
	if n := countSubmissions(t, db); n != 1 {
		t.Errorf("submissions count = %d, want 1 (the capacity-rejected attempt must never reach submission.Admit)", n)
	}
}

// TestSharedLimiter_KubernetesAndCloudTrailEndpoints_ShareOneCapacityBudget
// is the decisive proof for the approved admission-control design (design
// report §D): one Limiter instance, constructed once and wired into both
// intake endpoints exactly as cmd/platform/main.go does, enforces one
// platform-wide budget. Two independent limiters would let this second
// admission through instead of rejecting it.
func TestSharedLimiter_KubernetesAndCloudTrailEndpoints_ShareOneCapacityBudget(t *testing.T) {
	db := testutil.MigratedPostgres(t)
	limiter := NewSlidingWindowLimiter(1, time.Second, nil)
	k8sSrv := newTestServerWithLimiter(t, db, limiter)
	ctSrv := newCloudTrailTestServerWithLimiter(t, db, limiter)

	resp1, decoded1 := postJSON(t, k8sSrv, `{"items":[{"auditID":"shared-1","stage":"ResponseComplete"}]}`, testToken)
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("kubernetes admission status = %d, want 200", resp1.StatusCode)
	}
	results, _ := decoded1["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results = %v, want 1 entry", results)
	}
	if _, hasID := results[0].(map[string]any)["id"]; !hasID {
		t.Fatalf("results[0] = %v, want an id -- the kubernetes admission must succeed and consume the shared budget", results[0])
	}

	resp2, decoded2 := postJSON(t, ctSrv, `{"eventID":"shared-2"}`, testToken)
	if resp2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("cloudtrail status after kubernetes exhausted the shared one-submission budget = %d, want 429", resp2.StatusCode)
	}
	if decoded2["error"] != "rejected: over capacity" {
		t.Errorf("error = %v, want a capacity-specific message", decoded2["error"])
	}
	if n := countSubmissions(t, db); n != 1 {
		t.Errorf("submissions count = %d, want 1 -- only the kubernetes admission, never the cloudtrail one", n)
	}
}
