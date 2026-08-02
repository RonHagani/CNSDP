// Package datasources implements the ingestion-channel summary module
// (ARCH-01 §2 module 10): a read-only, retrospective account of telemetry
// already admitted through the platform's defined intake (FR-036). It owns
// no artifact table of its own -- like internal/evidence and
// internal/traceability, it reads submissions only through
// internal/submission's sanctioned read API.
//
// This module reports counts and timing only. It deliberately carries no
// health, staleness, or missing-delivery judgment, and no concept of
// multiple independently identified sources -- see docs/scope.md scope
// decision 6, as clarified for this module.
package datasources

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"cnsdp/internal/auth"
	"cnsdp/internal/diagnostics"
	"cnsdp/internal/submission"
)

// auditEventsID identifies the platform's one real ingestion channel
// (POST /v1/audit-events, internal/intake). v0.1 has no mechanism for
// multiple independently identified sources.
const auditEventsID = "audit-events"

// DataSource is a retrospective summary of telemetry already admitted
// through one ingestion channel.
type DataSource struct {
	ID          string
	DisplayName string
	Endpoint    string
	EventCount  int64
	LastEventAt *time.Time
}

// List returns the platform's data sources. v0.1 always returns exactly
// the one channel that exists, whether or not it has admitted anything yet.
func List(ctx context.Context, db submission.DB) ([]DataSource, error) {
	count, lastEventAt, err := submission.AdmittedSummary(ctx, db)
	if err != nil {
		return nil, err
	}
	return []DataSource{{
		ID:          auditEventsID,
		DisplayName: "Audit Events API",
		Endpoint:    "POST /v1/audit-events",
		EventCount:  count,
		LastEventAt: lastEventAt,
	}}, nil
}

// Handler serves the data sources endpoint (GET /v1/data-sources).
type Handler struct {
	DB    *sql.DB
	Token string
}

type dataSourceItem struct {
	ID          string     `json:"id"`
	DisplayName string     `json:"displayName"`
	Endpoint    string     `json:"endpoint"`
	EventCount  int64      `json:"eventCount"`
	LastEventAt *time.Time `json:"lastEventAt"`
}

type listResponse struct {
	DataSources []dataSourceItem `json:"dataSources"`
	Total       int              `json:"total"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !auth.Bearer(r.Header.Get("Authorization"), h.Token) {
		diagnostics.LogAccessDenied(r)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	sources, err := List(r.Context(), h.DB)
	if err != nil {
		slog.Error("datasources: list failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	items := make([]dataSourceItem, len(sources))
	for i, s := range sources {
		items[i] = dataSourceItem{
			ID:          s.ID,
			DisplayName: s.DisplayName,
			Endpoint:    s.Endpoint,
			EventCount:  s.EventCount,
			LastEventAt: s.LastEventAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(listResponse{DataSources: items, Total: len(items)}); err != nil {
		slog.Error("datasources: encode response failed", "error", err)
	}
}
