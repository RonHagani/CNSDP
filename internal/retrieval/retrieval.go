// Package retrieval implements the retrieval and investigation module
// (ARCH-01 §2 module 8): a bearer-token-authenticated HTTP endpoint that
// walks the traceability chain from a generated alert back to its source
// event (FR-030, FR-034), consistent with UC-003's main flow -- the
// analyst's entry point is the alert, not the submission. It owns no
// artifact table of its own: the response is composed entirely through
// internal/evidence.Compose, which itself reads only through each owning
// module's sanctioned API. This is a read-only path -- no write, no
// transaction.
package retrieval

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"cnsdp/internal/alerting"
	"cnsdp/internal/auth"
	"cnsdp/internal/detection"
	"cnsdp/internal/evidence"
	"cnsdp/internal/normalization"
)

// Handler serves the alert retrieval endpoint.
type Handler struct {
	DB    *sql.DB
	Token string
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	if !auth.Bearer(r.Header.Get("Authorization"), h.Token) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	alertID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || alertID <= 0 {
		http.Error(w, "alert id must be a positive integer", http.StatusBadRequest)
		return
	}

	inv, err := evidence.Compose(r.Context(), h.DB, alertID)
	if errors.Is(err, evidence.ErrNotFound) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		slog.Error("retrieval: compose alert inventory failed", "alert_id", alertID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(toResponse(inv)); err != nil {
		// The response status and headers are already committed at this
		// point, so this can only be logged, never turned into an error
		// response (NFR-022: never conflate a platform fault with a
		// product outcome already sent to the client).
		slog.Error("retrieval: encode response failed", "alert_id", alertID, "error", err)
	}
}

// response is the wire shape for a successful retrieval (FR-030, FR-034,
// FR-035): the alert summary, the six-artifact evidence inventory with
// explicit per-artifact availability, and the traceability verification
// result. Owned entirely by this package -- internal/evidence's Inventory
// is an internal composition type, not a wire format, so the raw event is
// converted here from []byte to json.RawMessage (embedded JSON, never
// base64) to preserve FR-032's evidentiary fidelity on the wire.
type response struct {
	AlertID int64 `json:"alertId"`

	SourceEvent         sourceEventField       `json:"sourceEvent"`
	ValidationOutcome   validationOutcomeField `json:"validationOutcome"`
	NormalizedEvent     normalizedEventField   `json:"normalizedEvent"`
	DetectionDefinition definitionField        `json:"detectionDefinition"`
	DetectionResult     detectionResultField   `json:"detectionResult"`
	Alert               alertField             `json:"alert"`

	Traceability traceabilityField `json:"traceability"`
}

type sourceEventField struct {
	Available bool            `json:"available"`
	RawEvent  json.RawMessage `json:"rawEvent,omitempty"`
}

type validationOutcomeField struct {
	Available bool   `json:"available"`
	Outcome   string `json:"outcome,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type normalizedEventField struct {
	Available bool                 `json:"available"`
	Event     *normalization.Event `json:"event,omitempty"`
}

type definitionField struct {
	Available  bool                  `json:"available"`
	Revision   string                `json:"revision,omitempty"`
	Definition *detection.Definition `json:"definition,omitempty"`
}

type detectionResultField struct {
	Available   bool                   `json:"available"`
	MatchReason *detection.MatchReason `json:"matchReason,omitempty"`
}

type alertField struct {
	Available bool              `json:"available"`
	Summary   *alerting.Summary `json:"summary,omitempty"`
}

type traceabilityField struct {
	Intact     bool   `json:"intact"`
	FailedLink string `json:"failedLink,omitempty"`
}

// toResponse converts an evidence.Inventory into the wire response.
// Fields are populated only when their corresponding Available flag is
// set, so an unavailable artifact is represented by available:false and
// an omitted content field, never a zero-valued placeholder that could be
// mistaken for real content.
func toResponse(inv *evidence.Inventory) response {
	resp := response{
		AlertID: inv.AlertID,
		Traceability: traceabilityField{
			Intact:     inv.Chain.Intact,
			FailedLink: inv.Chain.FailedLink,
		},
	}

	resp.SourceEvent.Available = inv.SourceEventAvailable
	if inv.SourceEventAvailable {
		resp.SourceEvent.RawEvent = json.RawMessage(inv.RawEvent)
	}

	resp.ValidationOutcome.Available = inv.ValidationOutcomeAvailable
	if inv.ValidationOutcomeAvailable {
		resp.ValidationOutcome.Outcome = string(inv.ValidationOutcome.Outcome)
		resp.ValidationOutcome.Reason = inv.ValidationOutcome.Reason
	}

	resp.NormalizedEvent.Available = inv.NormalizedEventAvailable
	if inv.NormalizedEventAvailable {
		event := inv.NormalizedEvent
		resp.NormalizedEvent.Event = &event
	}

	resp.DetectionDefinition.Available = inv.DefinitionAvailable
	if inv.DefinitionAvailable {
		def := inv.Definition
		resp.DetectionDefinition.Revision = inv.DefinitionRevision
		resp.DetectionDefinition.Definition = &def
	}

	resp.DetectionResult.Available = inv.DetectionResultAvailable
	if inv.DetectionResultAvailable {
		reason := inv.MatchReason
		resp.DetectionResult.MatchReason = &reason
	}

	resp.Alert.Available = inv.AlertAvailable
	if inv.AlertAvailable {
		summary := inv.AlertSummary
		resp.Alert.Summary = &summary
	}

	return resp
}
