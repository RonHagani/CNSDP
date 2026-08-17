// Package intake implements the telemetry admission module (ARCH-01 §2
// module 1): a Kubernetes audit-webhook-compatible HTTP(S) endpoint
// (ADR-0003) that authenticates a shared bearer token (NFR-012, NFR-013)
// and durably admits every item of a structurally valid EventList as a
// submission. It performs no content-shape classification -- FR-005's
// four-way validation outcome is a later checkpoint's responsibility, not
// this module's.
//
// Submissions offered above the documented v0.1 capacity envelope
// (NFR-003) are visibly rejected at admission, per submission, never
// silently accepted or dropped (NFR-004, NFR-013; AC-022) -- see
// admission.go for the limiter and its rationale.
package intake

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"

	"cnsdp/internal/auth"
	"cnsdp/internal/db"
	"cnsdp/internal/diagnostics"
	"cnsdp/internal/submission"
)

// DefaultMaxBodyBytes is used when no explicit MaxBodyBytes is configured.
// Baseline transport hardening (a conservative request-size bound), not a
// stand-in for NFR-003's capacity/admission-control envelope.
const DefaultMaxBodyBytes = 10 << 20 // 10 MiB

// Handler serves the intake endpoint. DB is an interface (satisfied by
// *sql.DB) rather than the concrete type so tests can inject a
// fault-injecting wrapper without mocking Postgres itself.
type Handler struct {
	DB           submission.DB
	Token        string
	MaxBodyBytes int64
	// Limiter gates per-submission admission against the approved v0.1
	// capacity envelope (NFR-003, NFR-004; AC-022). A nil Limiter applies
	// no admission control -- every existing construction site (tests,
	// and any future caller that doesn't set it) keeps its current
	// unrestricted behavior unchanged. cmd/platform wires a real
	// *SlidingWindowLimiter in production.
	Limiter Limiter
}

type admissionResult struct {
	Index int    `json:"index"`
	ID    int64  `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !auth.Bearer(r.Header.Get("Authorization"), h.Token) {
		diagnostics.LogAccessDenied(r)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	maxBodyBytes := h.MaxBodyBytes
	if maxBodyBytes <= 0 {
		maxBodyBytes = DefaultMaxBodyBytes
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "unable to read request body", http.StatusBadRequest)
		}
		return
	}

	items, err := parseEnvelope(body)
	if err != nil {
		http.Error(w, "malformed EventList envelope: "+err.Error(), http.StatusBadRequest)
		return
	}

	results := make([]admissionResult, len(items))
	var anyFailure, anyConflict bool
	capacityRejected := 0
	for i, raw := range items {
		// Admission is gated per submission, not per HTTP request: a
		// batch may contain both admitted and capacity-rejected items
		// (NFR-003, NFR-004; AC-022). A capacity-rejected item never
		// reaches submission.Admit -- no submissions row, and therefore
		// no validation_outcomes row, can ever be created for it.
		if h.Limiter != nil && !h.Limiter.Allow() {
			results[i] = admissionResult{Index: i, Error: "rejected: over capacity"}
			capacityRejected++
			continue
		}

		auditID, stage := extractIdentity(raw)
		id, err := submission.Admit(r.Context(), h.DB, bytes.Clone(raw), auditID, stage)
		switch {
		case errors.Is(err, submission.ErrSourceConflict):
			results[i] = admissionResult{Index: i, Error: "conflict: an existing submission with different content already uses this source identity"}
			anyConflict = true
		case db.IsResourceExhausted(err):
			// The documented persistent-storage resource limit was reached
			// mid-write (NFR-036; AC-023) -- a distinct, diagnosable
			// platform-fault outcome, not an undifferentiated internal
			// error. The HTTP response shape is unchanged (still a generic
			// "internal error" under the existing 503 path below): only the
			// structured log (diagnostics.LogResourceExhausted, the single
			// log statement for this event -- matching LogAccessDenied/
			// LogCapacityRejected's own single-call convention) gains the
			// distinguishing classification.
			diagnostics.LogResourceExhausted(r, i)
			results[i] = admissionResult{Index: i, Error: "internal error"}
			anyFailure = true
		case err != nil:
			slog.Error("intake: admit failed", "index", i, "error", err)
			results[i] = admissionResult{Index: i, Error: "internal error"}
			anyFailure = true
		default:
			results[i] = admissionResult{Index: i, ID: id}
		}
	}

	status := http.StatusOK
	switch {
	case anyFailure:
		status = http.StatusServiceUnavailable
	case capacityRejected > 0:
		status = http.StatusTooManyRequests
	case anyConflict:
		status = http.StatusConflict
	}

	if capacityRejected > 0 {
		// Admission-and-security outcome (NFR-004), distinct from a
		// telemetry data-quality outcome and from the NFR-013 denied-access
		// outcome logged by diagnostics.LogAccessDenied above -- never
		// written to validation_outcomes (NFR-022). Logged whenever any
		// item was capacity-rejected, independent of the response's final
		// status below -- true and useful even if an unrelated failure
		// elsewhere in the batch ends up dominating that status.
		diagnostics.LogCapacityRejected(r, capacityRejected)
	}
	if status == http.StatusTooManyRequests {
		// Retry-After reflects the admission-control refill window
		// specifically and is only meaningful when capacity rejection is
		// the response's actual, dominant outcome (status == 429). A batch
		// where an unrelated platform fault also occurred reports 503
		// instead (anyFailure takes precedence above) and must not carry
		// a capacity-derived retry hint alongside an unrelated failure.
		w.Header().Set("Retry-After", "1")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"results": results})
}

// parseEnvelope requires the top-level JSON value to be an object with an
// explicitly present "items" field whose value is a JSON array:
//
//	{"items": []}         -> valid, empty batch
//	{}                    -> error: "items" missing
//	{"items": null}       -> error: "items" must not be null
//	{"items": {...}} etc. -> error: "items" must be an array
//
// Every item's original bytes are preserved untouched in the returned
// slice -- no re-marshal.
func parseEnvelope(body []byte) ([]json.RawMessage, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("top-level value must be a JSON object: %w", err)
	}

	itemsRaw, ok := envelope["items"]
	if !ok {
		return nil, errors.New(`"items" field is required`)
	}
	if bytes.Equal(bytes.TrimSpace(itemsRaw), []byte("null")) {
		return nil, errors.New(`"items" must not be null`)
	}

	var items []json.RawMessage
	if err := json.Unmarshal(itemsRaw, &items); err != nil {
		return nil, fmt.Errorf(`"items" must be an array: %w`, err)
	}
	return items, nil
}

// extractIdentity best-effort-decodes one raw item against the official
// audit/v1 Event type purely to pull auditID/stage for the two submissions
// columns. It never fails: a decode error (or an item that isn't
// Event-shaped at all) yields empty strings, deferring any "cannot be
// parsed" / "missing required field" judgment entirely to the validation
// checkpoint (FR-004, FR-008, FR-009 -- out of scope here).
func extractIdentity(raw json.RawMessage) (auditID, stage string) {
	var ev auditv1.Event
	if err := json.Unmarshal(raw, &ev); err != nil {
		return "", ""
	}
	return string(ev.AuditID), string(ev.Stage)
}
