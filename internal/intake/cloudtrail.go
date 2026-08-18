// CloudTrail intake (ARCH-01 §2 module 1, ADR-0006): a second, dedicated
// HTTP(S) endpoint accepting AWS CloudTrail management-event records, one
// per request body -- reusing ADR-0003's dual-layer intake pattern
// (external contract mapped onto the same small canonical internal
// submission model) rather than re-deciding it, but with a single-record
// contract rather than a batch envelope: ADR-0006's bridge/poller delivers
// one CloudTrail record per queue message, so this endpoint's fixture and
// live wire shapes stay identical, unlike a batch contract nothing in
// production would ever actually send.
//
// Which endpoint a submission arrived through is itself the source-family
// attribution signal, recorded explicitly at admission time
// (submission.FamilyCloudTrail) -- ADR-0006 deliberately rejected
// content-sniffing on a unified endpoint.
package intake

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"cnsdp/internal/auth"
	"cnsdp/internal/db"
	"cnsdp/internal/diagnostics"
	"cnsdp/internal/submission"
)

// CloudTrailHandler serves the CloudTrail intake endpoint. Structurally
// parallel to Handler (shared auth/body-limit/admission-control
// mechanics), but not a shared/generalized type: the two source families'
// wire contracts (batched EventList vs. single record) are different
// enough that forcing them through one abstraction would cost more
// clarity than it would save, for exactly two families.
type CloudTrailHandler struct {
	DB           submission.DB
	Token        string
	MaxBodyBytes int64
	// Limiter is the same shared instance cmd/platform wires into the
	// Kubernetes Handler -- one platform-wide admission-control budget
	// (NFR-003, NFR-004) across both intake endpoints, since both feed the
	// same single-threaded worker and persistence pipeline (ADR-0002). A
	// second, independent limiter here would silently double the
	// documented capacity envelope.
	Limiter Limiter
}

type cloudTrailAdmissionResult struct {
	ID    int64  `json:"id,omitempty"`
	Error string `json:"error,omitempty"`
}

func (h *CloudTrailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	// Admission is gated the same way the Kubernetes endpoint gates each
	// item (NFR-003, NFR-004; AC-022) -- here, once per request, since one
	// request is exactly one submission. A capacity-rejected body never
	// reaches submission.Admit -- no submissions row, and therefore no
	// validation_outcomes row, is ever created for it.
	if h.Limiter != nil && !h.Limiter.Allow() {
		diagnostics.LogCapacityRejected(r, 1)
		w.Header().Set("Retry-After", "1")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(cloudTrailAdmissionResult{Error: "rejected: over capacity"})
		return
	}

	// Every syntactically-identifiable body is admitted regardless of
	// content shape (FR-001) -- no upfront JSON-validity gate here, mirroring
	// how a malformed item inside a Kubernetes EventList batch is still
	// admitted rather than rejected at intake. FR-005's four-way
	// classification (Unsupported for unparseable/non-Management content)
	// is the validation checkpoint's responsibility, not this one's.
	eventID := extractCloudTrailIdentity(body)
	id, err := submission.Admit(r.Context(), h.DB, bytes.Clone(body), submission.FamilyCloudTrail, "", "", eventID)

	w.Header().Set("Content-Type", "application/json")
	switch {
	case errors.Is(err, submission.ErrSourceConflict):
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(cloudTrailAdmissionResult{
			Error: "conflict: an existing submission with different content already uses this source identity",
		})
	case db.IsResourceExhausted(err):
		// The documented persistent-storage resource limit was reached
		// mid-write (NFR-036; AC-023) -- same distinguishable-platform-fault
		// treatment as the Kubernetes endpoint: the HTTP response stays a
		// generic "internal error", and only the structured log gains the
		// distinguishing classification.
		diagnostics.LogResourceExhausted(r, 0)
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(cloudTrailAdmissionResult{Error: "internal error"})
	case err != nil:
		slog.Error("intake: cloudtrail admit failed", "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(cloudTrailAdmissionResult{Error: "internal error"})
	default:
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(cloudTrailAdmissionResult{ID: id})
	}
}

// extractCloudTrailIdentity best-effort-decodes body purely to pull the
// record's own eventID for the submissions.source_event_id column. It
// never fails: a decode error (or a body that isn't record-shaped at all)
// yields an empty string, deferring any "cannot be parsed" / "missing
// required field" judgment entirely to the validation checkpoint (FR-004,
// FR-008, FR-009 -- out of scope here) -- the same never-fails contract
// extractIdentity already provides for Kubernetes items.
func extractCloudTrailIdentity(raw []byte) (eventID string) {
	var rec struct {
		EventID string `json:"eventID"`
	}
	if err := json.Unmarshal(raw, &rec); err != nil {
		return ""
	}
	return rec.EventID
}
