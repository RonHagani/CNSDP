package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// APIClient talks to the platform exclusively through its real,
// product-exposed HTTP interface -- the same path any real client uses.
// Every state-changing action the harness performs against the product
// goes through PostAuditEvents; every other method is a read.
type APIClient struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func NewAPIClient(baseURL, token string) *APIClient {
	return &APIClient{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

type intakeResult struct {
	Index int    `json:"index"`
	ID    int64  `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

type intakeResponse struct {
	Results []intakeResult `json:"results"`
}

// PostAuditEvent submits exactly one EventList item as its own HTTP
// request -- offered-rate control in the sustained-run phase maps one
// HTTP POST to one "submission attempt", matching AC-022's own unit of
// measure unambiguously, rather than leaving "attempt" ambiguous over a
// batched request. Returns the admitted submission id.
func (c *APIClient) PostAuditEvent(ctx context.Context, item json.RawMessage) (submissionID int64, statusCode int, err error) {
	envelope, err := json.Marshal(map[string]any{"items": []json.RawMessage{item}})
	if err != nil {
		return 0, 0, fmt.Errorf("marshal envelope: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/audit-events", bytes.NewReader(envelope))
	if err != nil {
		return 0, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, resp.StatusCode, fmt.Errorf("read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, resp.StatusCode, fmt.Errorf("intake returned %d: %s", resp.StatusCode, truncate(body, 500))
	}

	var out intakeResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, resp.StatusCode, fmt.Errorf("unmarshal intake response: %w", err)
	}
	if len(out.Results) != 1 {
		return 0, resp.StatusCode, fmt.Errorf("intake response had %d results, want 1", len(out.Results))
	}
	if out.Results[0].Error != "" {
		return 0, resp.StatusCode, fmt.Errorf("intake rejected item: %s", out.Results[0].Error)
	}
	return out.Results[0].ID, resp.StatusCode, nil
}

// TimedGet issues an authenticated GET against path and returns the
// wall-clock duration from request start to full response-body read
// (matching AC-021's "retrieval... completes within N seconds" -- the
// client-observed completion time, not merely a byte from the server) and
// the resulting status code. It reads and discards the body: AC-021
// measures retrieval time, not content, and every phase that needs
// specific content (correlation ids) uses direct SELECT reads against the
// authoritative database instead (see dbaccess.go).
func (c *APIClient) TimedGet(ctx context.Context, path string) (elapsed time.Duration, statusCode int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	start := time.Now()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return time.Since(start), 0, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	_, readErr := io.Copy(io.Discard, resp.Body)
	elapsed = time.Since(start)
	if readErr != nil {
		return elapsed, resp.StatusCode, fmt.Errorf("read response body: %w", readErr)
	}
	return elapsed, resp.StatusCode, nil
}

// Ready reports whether GET /readyz currently returns 200 -- the
// documented readiness determination (NFR-020), used for both initial
// startup and post-recovery RTO measurement (AC-019). Unauthenticated by
// design (internal/diagnostics), so no bearer token is sent.
func (c *APIClient) Ready(ctx context.Context) bool {
	ok, _ := c.ReadyDiag(ctx)
	return ok
}

// ReadyDiag is Ready plus a short, human-readable reason for a negative
// result (a transport error, a non-200 status, or the readyz body's own
// failed_check field) -- used only for the recovery phase's live progress
// logging so a long wait is diagnosable in real time instead of a silent
// timeout with no clue why.
func (c *APIClient) ReadyDiag(ctx context.Context) (bool, string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/readyz", nil)
	if err != nil {
		return false, "build request: " + err.Error()
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return false, "transport error: " + err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if resp.StatusCode == http.StatusOK {
		return true, ""
	}
	return false, fmt.Sprintf("status %d: %s", resp.StatusCode, truncate(body, 200))
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "...(truncated)"
}
