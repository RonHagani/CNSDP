package retrieval

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"cnsdp/internal/auth"
	"cnsdp/internal/diagnostics"
	"cnsdp/internal/evidence"
	"cnsdp/internal/normalization"
)

// ListHandler serves the alert inventory endpoint (GET /v1/alerts): a
// deterministic, ordered summary of every persisted alert, built for the
// alert-inventory list screen rather than for single-alert investigation
// (Handler, GET /v1/alerts/{id}). It never returns the full six-artifact
// investigation payload -- each row carries only the fields
// evidence.SummaryItem already provides.
type ListHandler struct {
	DB    *sql.DB
	Token string
}

func (h *ListHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !auth.Bearer(r.Header.Get("Authorization"), h.Token) {
		diagnostics.LogAccessDenied(r)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	items, err := evidence.ComposeList(r.Context(), h.DB)
	if err != nil {
		slog.Error("retrieval: compose alert list failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(toListResponse(items)); err != nil {
		// The response status and headers are already committed at this
		// point (NFR-022: never conflate a platform fault with a product
		// outcome already sent to the client).
		slog.Error("retrieval: encode list response failed", "error", err)
	}
}

// listResponse is the wire shape for a successful GET /v1/alerts (the
// list-level analog of response): a stably ordered array of alert
// summaries plus the total count.
type listResponse struct {
	Alerts []alertSummaryItem `json:"alerts"`
	Total  int                `json:"total"`
}

// alertSummaryItem is one inventory row's wire shape -- a dedicated
// summary contract, distinct from response (the single-alert investigation
// payload), so a list of many alerts never embeds any row's raw event,
// normalized event, or detection definition content.
type alertSummaryItem struct {
	AlertID       int64                   `json:"alertId"`
	DetectionName string                  `json:"detectionName"`
	Subject       normalization.Subject   `json:"subject"`
	Operation     normalization.Operation `json:"operation"`
	Target        normalization.Target    `json:"target"`
	Outcome       normalization.Outcome   `json:"outcome"`
	RequestTime   time.Time               `json:"requestTime"`
	Traceability  traceabilityField       `json:"traceability"`
}

func toListResponse(items []evidence.SummaryItem) listResponse {
	alerts := make([]alertSummaryItem, 0, len(items))
	for _, it := range items {
		alerts = append(alerts, alertSummaryItem{
			AlertID:       it.AlertID,
			DetectionName: it.Summary.MatchReason.DefinitionName,
			Subject:       it.Summary.Subject,
			Operation:     it.Summary.Operation,
			Target:        it.Summary.Target,
			Outcome:       it.Summary.Outcome,
			RequestTime:   it.Summary.RequestTime,
			Traceability: traceabilityField{
				Intact:     it.Chain.Intact,
				FailedLink: it.Chain.FailedLink,
			},
		})
	}
	return listResponse{Alerts: alerts, Total: len(alerts)}
}
