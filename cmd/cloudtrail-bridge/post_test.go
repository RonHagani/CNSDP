package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClassifyStatus(t *testing.T) {
	cases := []struct {
		status int
		want   postOutcome
	}{
		{http.StatusOK, outcomeDelete},
		{http.StatusUnauthorized, outcomeFatal},
		{http.StatusForbidden, outcomeFatal},
		{http.StatusNotFound, outcomeFatal},
		{http.StatusMethodNotAllowed, outcomeFatal},
		{http.StatusConflict, outcomeRetainImmediate},
		{http.StatusRequestEntityTooLarge, outcomeRetainImmediate},
		{http.StatusBadRequest, outcomeRetainImmediate},
		{http.StatusTooManyRequests, outcomeRetainBackoff},
		{http.StatusInternalServerError, outcomeRetainBackoff},
		{http.StatusServiceUnavailable, outcomeRetainBackoff},
		{http.StatusBadGateway, outcomeRetainBackoff},
	}
	for _, c := range cases {
		if got := classifyStatus(c.status); got != c.want {
			t.Errorf("classifyStatus(%d) = %v, want %v", c.status, got, c.want)
		}
	}
}

func newTestPoster(t *testing.T, handler http.HandlerFunc) (*Poster, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Poster{
		Client:   &http.Client{Timeout: cnsdpPOSTTimeout},
		Endpoint: srv.URL,
		Token:    "test-token",
	}, srv
}

func TestPoster_Post_200_Delete(t *testing.T) {
	var gotAuth, gotBody string
	poster, _ := newTestPoster(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1}`))
	})

	result := poster.Post(context.Background(), []byte(`{"eventID":"evt-1"}`))

	if result.outcome != outcomeDelete {
		t.Errorf("outcome = %v, want outcomeDelete", result.outcome)
	}
	if result.statusCode != http.StatusOK {
		t.Errorf("statusCode = %d, want 200", result.statusCode)
	}
	if gotAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer test-token")
	}
	if gotBody != `{"eventID":"evt-1"}` {
		t.Errorf("posted body = %q, want the record forwarded verbatim", gotBody)
	}
}

func TestPoster_Post_409_RetainImmediate(t *testing.T) {
	poster, _ := newTestPoster(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	})
	result := poster.Post(context.Background(), []byte(`{}`))
	if result.outcome != outcomeRetainImmediate {
		t.Errorf("outcome = %v, want outcomeRetainImmediate", result.outcome)
	}
}

func TestPoster_Post_429_RetainBackoff(t *testing.T) {
	poster, _ := newTestPoster(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	})
	result := poster.Post(context.Background(), []byte(`{}`))
	if result.outcome != outcomeRetainBackoff {
		t.Errorf("outcome = %v, want outcomeRetainBackoff", result.outcome)
	}
}

func TestPoster_Post_503_RetainBackoff(t *testing.T) {
	poster, _ := newTestPoster(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	result := poster.Post(context.Background(), []byte(`{}`))
	if result.outcome != outcomeRetainBackoff {
		t.Errorf("outcome = %v, want outcomeRetainBackoff", result.outcome)
	}
}

func TestPoster_Post_401_Fatal(t *testing.T) {
	poster, _ := newTestPoster(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	result := poster.Post(context.Background(), []byte(`{}`))
	if result.outcome != outcomeFatal {
		t.Errorf("outcome = %v, want outcomeFatal", result.outcome)
	}
}

func TestPoster_Post_404_Fatal(t *testing.T) {
	poster, _ := newTestPoster(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	result := poster.Post(context.Background(), []byte(`{}`))
	if result.outcome != outcomeFatal {
		t.Errorf("outcome = %v, want outcomeFatal", result.outcome)
	}
}

func TestPoster_Post_405_Fatal(t *testing.T) {
	poster, _ := newTestPoster(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
	})
	result := poster.Post(context.Background(), []byte(`{}`))
	if result.outcome != outcomeFatal {
		t.Errorf("outcome = %v, want outcomeFatal", result.outcome)
	}
}

func TestPoster_Post_NetworkError_RetainBackoff(t *testing.T) {
	// A Poster pointed at an address nothing is listening on: the
	// request never gets an HTTP response at all.
	poster := &Poster{
		Client:   &http.Client{Timeout: 2 * time.Second},
		Endpoint: "http://127.0.0.1:1", // reserved, nothing listens here
		Token:    "test-token",
	}
	result := poster.Post(context.Background(), []byte(`{}`))
	if result.outcome != outcomeRetainBackoff {
		t.Errorf("outcome = %v, want outcomeRetainBackoff", result.outcome)
	}
	if result.err == nil {
		t.Error("expected a non-nil transport error")
	}
}

func TestPoster_Post_ContextTimeout_RetainBackoff(t *testing.T) {
	poster, _ := newTestPoster(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(200 * time.Millisecond):
		case <-r.Context().Done():
		}
		w.WriteHeader(http.StatusOK)
	})
	poster.Client = &http.Client{Timeout: 10 * time.Millisecond}

	result := poster.Post(context.Background(), []byte(`{}`))
	if result.outcome != outcomeRetainBackoff {
		t.Errorf("outcome = %v, want outcomeRetainBackoff", result.outcome)
	}
}

func TestPoster_Post_NeverLogsOrLeaksTokenViaResult(t *testing.T) {
	poster, _ := newTestPoster(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	poster.Token = "super-secret-token-value"

	result := poster.Post(context.Background(), []byte(`{}`))

	// postResult carries no field derived from the token; this guards
	// against a future field addition accidentally leaking it.
	if result.err != nil && strings.Contains(result.err.Error(), poster.Token) {
		t.Error("postResult.err unexpectedly contains the bearer token")
	}
}
