package diagnostics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestServeHTTP_WrongMethod_Returns405 exercises the one branch that
// needs no database at all: the method check runs before h.DB is ever
// touched.
func TestServeHTTP_WrongMethod_Returns405(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/readyz", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
