// Package diagnostics implements the operational diagnostics module
// (ARCH-01 §2 module 9): the platform readiness endpoint (NFR-020) and
// shared structured-logging helpers for denied-access and capacity-
// rejected admission-and-security outcomes on every authenticated
// product-exposed path (NFR-019, NFR-021, NFR-022).
//
// It owns no artifact table: readiness is a live check against existing
// state, never a persisted product artifact, and reads that state only
// through the owning module's sanctioned API (internal/detection's
// ActiveDefinitions), never by querying another module's table directly.
package diagnostics

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"cnsdp/internal/detection"
)

// Handler serves the readiness endpoint (GET /readyz). Unauthenticated by
// design: NFR-012/AC-024 bind access to product data and functions --
// telemetry, normalized events, detection definitions, match reasons,
// alerts, evidence, validation outcomes -- none of which readiness
// exposes; it reports operational status only.
type Handler struct {
	DB *sql.DB
}

type readyResponse struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

type notReadyResponse struct {
	Status      string `json:"status"`
	FailedCheck string `json:"failed_check"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	failedCheck := h.firstFailedCheck(r.Context())

	w.Header().Set("Content-Type", "application/json")

	if failedCheck != "" {
		// A failed readiness check is an expected, actionable operational
		// signal (NFR-020), not a handler bug -- logged at Warn, and only
		// on failure, so a frequently-polling orchestrator does not fill
		// the log with routine successful probes.
		slog.Warn("diagnostics: readiness check failed", "failed_check", failedCheck)
		w.WriteHeader(http.StatusServiceUnavailable)
		if err := json.NewEncoder(w).Encode(notReadyResponse{Status: "not_ready", FailedCheck: failedCheck}); err != nil {
			slog.Error("diagnostics: encode readiness response failed", "error", err)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(readyResponse{
		Status: "ready",
		Checks: map[string]string{"database": "ok", "detection_definitions": "ok"},
	}); err != nil {
		slog.Error("diagnostics: encode readiness response failed", "error", err)
	}
}

// firstFailedCheck runs both readiness checks in order and returns the
// name of the first one that fails, or "" if both pass. Neither check's
// underlying error (a driver error or internal detail) is ever included
// in the response or propagated to the caller -- only the check's name.
//
//   - "database": PostgreSQL reachability (PingContext).
//   - "detection_definitions": the complete active version-controlled
//     detection-definition set resolves (detection.ActiveDefinitions) --
//     database reachability alone does not prove the platform can
//     process admitted telemetry.
func (h *Handler) firstFailedCheck(ctx context.Context) string {
	if err := h.DB.PingContext(ctx); err != nil {
		return "database"
	}
	if _, err := detection.ActiveDefinitions(ctx, h.DB); err != nil {
		return "detection_definitions"
	}
	return ""
}

// LogAccessDenied records one denied-access attempt at an authenticated
// product-exposed path (NFR-019, AC-026): safe request metadata only --
// method, path, and the admission-and-security outcome family (NFR-022)
// -- never the bearer token, request body, raw telemetry, or internal
// database error text. Shared by internal/intake and internal/retrieval
// so denied-access logging cannot drift out of sync between them.
func LogAccessDenied(r *http.Request) {
	slog.Warn("diagnostics: denied access attempt",
		"method", r.Method,
		"path", r.URL.Path,
		"outcome_family", "admission_security",
	)
}

// LogCapacityRejected records one HTTP response in which one or more
// submission attempts were denied at admission for exceeding the approved
// v0.1 capacity envelope (NFR-004, NFR-013; AC-022): the same
// admission-and-security outcome family as LogAccessDenied, distinguished
// by reason rather than by family (NFR-022) -- never a telemetry
// data-quality outcome, and never written to validation_outcomes. Safe
// request metadata and the rejected-item count only -- never the bearer
// token, request body, or raw telemetry content (NFR-015).
func LogCapacityRejected(r *http.Request, rejectedCount int) {
	slog.Warn("diagnostics: capacity-rejected submission attempt(s)",
		"method", r.Method,
		"path", r.URL.Path,
		"outcome_family", "admission_security",
		"reason", "capacity",
		"rejected_count", rejectedCount,
	)
}

// LogResourceExhausted records one admission write that failed because the
// documented persistent-storage resource limit was reached (NFR-036;
// AC-023): a platform-fault outcome, distinct from both the data-quality
// outcomes validation_outcomes records and the admission-security outcomes
// LogAccessDenied/LogCapacityRejected report above (NFR-022). Never
// written to validation_outcomes. Safe request metadata and the failed
// item's index only -- never the bearer token, request body, raw
// telemetry, or the underlying driver error text (NFR-015).
func LogResourceExhausted(r *http.Request, index int) {
	slog.Error("diagnostics: admission write failed: resource exhausted",
		"method", r.Method,
		"path", r.URL.Path,
		"outcome_family", "platform_fault",
		"error_family", "resource_exhausted",
		"index", index,
	)
}
