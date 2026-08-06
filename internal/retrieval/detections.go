package retrieval

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"

	"cnsdp/internal/auth"
	"cnsdp/internal/detection"
	"cnsdp/internal/diagnostics"
)

// DetectionsHandler serves the detections catalog endpoint (GET
// /v1/detections): the currently active detection definitions (FR-020,
// FR-021, FR-022) -- never a historical, superseded, or inactive revision
// (ADR-0004: no code path exists to edit or manage a definition through the
// product itself). It owns no query or loading logic of its own; every
// definition comes from detection.ActiveDefinitions, the same canonical
// path GET /v1/alerts/{id} already resolves a matched definition through.
type DetectionsHandler struct {
	DB    *sql.DB
	Token string
}

// detectionItem is one catalog row's wire shape: the canonical Definition
// content (scenario, name, description, conditions), embedded unchanged,
// plus the revision identifying which immutable, version-controlled
// revision is currently active (NFR-025).
type detectionItem struct {
	detection.Definition
	Revision string `json:"revision"`
}

// detectionsListResponse is the wire shape for a successful GET
// /v1/detections: the active definitions in the deterministic order
// detection.ActiveDefinitions already returns them (scenario-1,
// scenario-2, scenario-3 -- alphabetical filename order, per
// loadDefinitionsFromFS), plus the total count.
type detectionsListResponse struct {
	Detections []detectionItem `json:"detections"`
	Total      int             `json:"total"`
}

func (h *DetectionsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !auth.Bearer(r.Header.Get("Authorization"), h.Token) {
		diagnostics.LogAccessDenied(r)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	active, err := detection.ActiveDefinitions(r.Context(), h.DB)
	if err != nil {
		slog.Error("retrieval: list active detections failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	items := make([]detectionItem, len(active))
	for i, a := range active {
		items[i] = detectionItem{Definition: a.Definition, Revision: a.Revision}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(detectionsListResponse{Detections: items, Total: len(items)}); err != nil {
		// The response status and headers are already committed at this
		// point (NFR-022: never conflate a platform fault with a product
		// outcome already sent to the client).
		slog.Error("retrieval: encode detections response failed", "error", err)
	}
}
