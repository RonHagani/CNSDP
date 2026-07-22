package intake

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseEnvelope_MultiItemArray(t *testing.T) {
	items, err := parseEnvelope([]byte(`{"items":[{"a":1},{"b":2}]}`))
	if err != nil {
		t.Fatalf("parseEnvelope: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if string(items[0]) != `{"a":1}` || string(items[1]) != `{"b":2}` {
		t.Errorf("items = %q, %q, want original raw bytes preserved", items[0], items[1])
	}
}

func TestParseEnvelope_MissingItemsField(t *testing.T) {
	_, err := parseEnvelope([]byte(`{}`))
	if err == nil {
		t.Fatal("expected an error for a missing \"items\" field")
	}
}

func TestParseEnvelope_ItemsNull(t *testing.T) {
	_, err := parseEnvelope([]byte(`{"items": null}`))
	if err == nil {
		t.Fatal("expected an error for \"items\": null")
	}
}

func TestParseEnvelope_NonArrayItems(t *testing.T) {
	cases := []string{
		`{"items": {"a":1}}`,
		`{"items": "oops"}`,
		`{"items": 42}`,
	}
	for _, c := range cases {
		if _, err := parseEnvelope([]byte(c)); err == nil {
			t.Errorf("parseEnvelope(%s): expected an error for non-array items", c)
		}
	}
}

func TestParseEnvelope_EmptyArrayIsValid(t *testing.T) {
	items, err := parseEnvelope([]byte(`{"items": []}`))
	if err != nil {
		t.Fatalf("parseEnvelope: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("len(items) = %d, want 0", len(items))
	}
}

func TestParseEnvelope_MalformedTopLevelJSON(t *testing.T) {
	cases := []string{
		`not json at all`,
		`[1,2,3]`,
		`"a string"`,
		`42`,
	}
	for _, c := range cases {
		if _, err := parseEnvelope([]byte(c)); err == nil {
			t.Errorf("parseEnvelope(%s): expected an error for a non-object top-level value", c)
		}
	}
}

func TestExtractIdentity_WellFormedEvent(t *testing.T) {
	auditID, stage := extractIdentity([]byte(`{"auditID":"a1","stage":"ResponseComplete"}`))
	if auditID != "a1" || stage != "ResponseComplete" {
		t.Errorf("got auditID=%q stage=%q, want a1/ResponseComplete", auditID, stage)
	}
}

func TestExtractIdentity_NonEventShapedItemReturnsEmpty(t *testing.T) {
	cases := []json.RawMessage{
		[]byte(`"not-an-object"`),
		[]byte(`42`),
		[]byte(`{"stage": 12345}`), // type mismatch on a string field
		[]byte(`{"foo":"bar"}`),    // valid object, but no identity fields
	}
	for _, c := range cases {
		auditID, stage := extractIdentity(c)
		if auditID != "" || stage != "" {
			t.Errorf("extractIdentity(%s) = %q/%q, want empty/empty", c, auditID, stage)
		}
	}
}

func TestAuthorized(t *testing.T) {
	const token = "secret-token"
	cases := []struct {
		name   string
		header string
		want   bool
	}{
		{"correct token", "Bearer secret-token", true},
		{"incorrect token", "Bearer wrong-token", false},
		{"missing header", "", false},
		{"malformed prefix", "secret-token", false},
		{"wrong scheme", "Basic secret-token", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := authorized(c.header, token); got != c.want {
				t.Errorf("authorized(%q, ...) = %v, want %v", c.header, got, c.want)
			}
		})
	}
}

type errorReader struct{ err error }

func (r errorReader) Read(p []byte) (int, error) { return 0, r.err }
func (r errorReader) Close() error               { return nil }

// TestServeHTTP_NonSizeBodyReadError_Returns400 proves a generic body-read
// failure (not http.MaxBytesReader's size-limit error) is reported as 400,
// not 413. Auth passes first, so the body-read path is what's exercised;
// h.DB is never reached from this path.
func TestServeHTTP_NonSizeBodyReadError_Returns400(t *testing.T) {
	h := &Handler{Token: "secret", MaxBodyBytes: 1024}
	req := httptest.NewRequest(http.MethodPost, "/v1/audit-events", errorReader{err: errors.New("boom")})
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServeHTTP_WrongMethod_Returns405(t *testing.T) {
	h := &Handler{Token: "secret", MaxBodyBytes: 1024}
	req := httptest.NewRequest(http.MethodGet, "/v1/audit-events", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
