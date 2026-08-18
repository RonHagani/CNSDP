package intake

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractCloudTrailIdentity_WellFormedRecord(t *testing.T) {
	eventID := extractCloudTrailIdentity([]byte(`{"eventID":"evt-1","eventName":"CreateAccessKey"}`))
	if eventID != "evt-1" {
		t.Errorf("got eventID=%q, want evt-1", eventID)
	}
}

func TestExtractCloudTrailIdentity_NonRecordShapedBodyReturnsEmpty(t *testing.T) {
	cases := [][]byte{
		[]byte(`"not-an-object"`),
		[]byte(`42`),
		[]byte(`{"eventID": 12345}`), // type mismatch on a string field
		[]byte(`{"foo":"bar"}`),      // valid object, but no identity field
		[]byte(`not json at all`),
		[]byte(``),
	}
	for _, c := range cases {
		if got := extractCloudTrailIdentity(c); got != "" {
			t.Errorf("extractCloudTrailIdentity(%s) = %q, want empty", c, got)
		}
	}
}

func TestCloudTrailServeHTTP_WrongMethod_Returns405(t *testing.T) {
	h := &CloudTrailHandler{Token: "secret", MaxBodyBytes: 1024}
	req := httptest.NewRequest(http.MethodGet, "/v1/cloudtrail-events", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

// TestCloudTrailServeHTTP_NonSizeBodyReadError_Returns400 mirrors
// TestServeHTTP_NonSizeBodyReadError_Returns400 (intake_test.go) for the
// CloudTrail endpoint: a generic body-read failure is reported 400, not
// 413. Auth passes first, so the body-read path is what's exercised; h.DB
// is never reached from this path.
func TestCloudTrailServeHTTP_NonSizeBodyReadError_Returns400(t *testing.T) {
	h := &CloudTrailHandler{Token: "secret", MaxBodyBytes: 1024}
	req := httptest.NewRequest(http.MethodPost, "/v1/cloudtrail-events", errorReader{err: errors.New("boom")})
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
