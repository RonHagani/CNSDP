// Package evidence implements the evidence inventory module (ARCH-01 §2
// module 6): FR-031 through FR-035's per-alert account of the six
// minimum-evidence-set artifacts. It owns no artifact table of its own --
// exactly like internal/traceability, every artifact it reads comes
// through the owning module's sanctioned read API (internal/submission,
// internal/validation, internal/normalization, internal/detection,
// internal/alerting, internal/traceability), never by querying another
// module's table directly.
//
// Two entry points build an Inventory, with deliberately different
// contracts: Advance is fail-fast -- it gates the alerted -> evidenced
// transition, so any missing artifact aborts the whole transaction and
// leaves the submission at alerted. Compose is best-effort -- it serves
// internal/retrieval's read-only presentation of an alert (FR-030,
// FR-034), so a missing or unavailable artifact is marked unavailable and
// composition continues, making the gap visible (FR-035) rather than
// turning it into an error. Compose must never be used to weaken or
// bypass Advance's gating contract, and Advance must never be relaxed to
// match Compose's tolerance.
package evidence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cnsdp/internal/alerting"
	"cnsdp/internal/detection"
	"cnsdp/internal/normalization"
	"cnsdp/internal/submission"
	"cnsdp/internal/traceability"
	"cnsdp/internal/validation"
)

// Inventory is the per-alert account of the six minimum-evidence-set
// artifacts locked for Checkpoint 9: the source submission's raw event,
// its persisted validation outcome, the normalized event it produced, the
// detection definition and detection result that matched, and the alert
// itself. The traceability chain-verification result (Chain) is
// additional metadata describing this inventory (FR-033, FR-034) -- it is
// not a seventh artifact and does not substitute for any of the six.
type Inventory struct {
	AlertID int64

	SourceEventAvailable       bool
	ValidationOutcomeAvailable bool
	NormalizedEventAvailable   bool
	DefinitionAvailable        bool
	DetectionResultAvailable   bool
	AlertAvailable             bool

	RawEvent           []byte
	ValidationOutcome  validation.Result
	NormalizedEvent    normalization.Event
	Definition         detection.Definition
	DefinitionRevision string
	MatchReason        detection.MatchReason
	AlertSummary       alerting.Summary

	Chain traceability.Result
}

// Complete reports whether every one of the six artifacts is available
// and the traceability chain resolved intact -- the gate Advance requires
// before a submission may advance to evidenced (FR-031, FR-035).
func (inv Inventory) Complete() bool {
	return inv.SourceEventAvailable &&
		inv.ValidationOutcomeAvailable &&
		inv.NormalizedEventAvailable &&
		inv.DefinitionAvailable &&
		inv.DetectionResultAvailable &&
		inv.AlertAvailable &&
		inv.Chain.Intact
}

// ErrInventoryIncomplete is returned by Advance when any alert's
// six-artifact inventory is incomplete or its traceability chain did not
// verify intact -- a missing artifact, a digest mismatch, or a
// source-key mismatch. The whole transaction is rolled back and the
// submission is left at alerted.
var ErrInventoryIncomplete = errors.New("evidence: alert inventory incomplete or traceability chain not intact")

// Advance verifies the complete six-artifact evidence inventory for every
// alert generated from sub's normalized event (FR-031 through FR-035) and
// advances the submission from alerted to evidenced -- all in one
// transaction (ADR-0002). Every artifact read happens through the open
// transaction, via the owning module's sanctioned read API, so
// verification and the status advance are atomic: on any missing
// artifact, digest mismatch, source-key mismatch, or read error, the
// whole transaction is rolled back and the submission remains at alerted.
//
// The alerted -> evidenced transition is unconditional: a submission with
// zero matching detection results has zero alerts, so verification
// succeeds vacuously (there is nothing to fail) and the submission still
// advances -- the same "status marks completed processing, not artifact
// existence" rule Checkpoint 8 established for evaluated -> alerted.
//
// Verification is all-or-nothing across every alert: if any one alert's
// inventory is incomplete or its chain does not verify intact, the whole
// transaction is rolled back for the submission as a whole, not just the
// failing alert -- mirroring every prior stage's all-or-nothing
// transaction behavior.
func Advance(ctx context.Context, db *sql.DB, sub *submission.Submission) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("evidence: begin advance tx for submission %d: %w", sub.ID, err)
	}
	defer tx.Rollback()

	rec, err := normalization.Get(ctx, tx, sub.ID)
	if err != nil {
		return fmt.Errorf("evidence: advance submission %d: %w", sub.ID, err)
	}

	matches, err := detection.MatchedResults(ctx, tx, rec.ID)
	if err != nil {
		return fmt.Errorf("evidence: advance submission %d: %w", sub.ID, err)
	}

	for _, m := range matches {
		inv, err := assembleInventory(ctx, tx, sub, rec, m)
		if err != nil {
			return fmt.Errorf("evidence: advance submission %d: %w", sub.ID, err)
		}
		if !inv.Complete() {
			return fmt.Errorf("%w: submission %d, detection result %d", ErrInventoryIncomplete, sub.ID, m.ID)
		}
	}

	if err := submission.Advance(ctx, tx, sub.ID, submission.StatusAlerted, submission.StatusEvidenced); err != nil {
		return fmt.Errorf("evidence: advance submission %d to evidenced: %w", sub.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("evidence: commit advance for submission %d: %w", sub.ID, err)
	}
	return nil
}

// assembleInventory composes one alert's six-artifact Inventory, reading
// each artifact exclusively through its owning module's sanctioned read
// API against the open transaction tx. It fails fast: the first missing
// artifact or read error aborts assembly and is propagated to Advance,
// which rolls back the whole transaction -- no partial Inventory is ever
// returned to a caller.
func assembleInventory(ctx context.Context, tx *sql.Tx, sub *submission.Submission, rec *normalization.Record, m detection.MatchedResult) (Inventory, error) {
	var inv Inventory

	inv.RawEvent = sub.RawEvent
	inv.SourceEventAvailable = len(sub.RawEvent) > 0

	vr, err := validation.Get(ctx, tx, sub.ID)
	if err != nil {
		return Inventory{}, fmt.Errorf("get validation outcome for submission %d: %w", sub.ID, err)
	}
	inv.ValidationOutcome = vr.Result
	inv.ValidationOutcomeAvailable = true

	inv.NormalizedEvent = rec.Event
	inv.NormalizedEventAvailable = true

	def, revision, err := detection.GetDefinition(ctx, tx, m.DetectionDefinitionID)
	if err != nil {
		return Inventory{}, fmt.Errorf("get detection definition %d: %w", m.DetectionDefinitionID, err)
	}
	inv.Definition = def
	inv.DefinitionRevision = revision
	inv.DefinitionAvailable = true

	inv.MatchReason = m.MatchReason
	inv.DetectionResultAvailable = true

	alert, err := alerting.Get(ctx, tx, m.ID)
	if err != nil {
		return Inventory{}, fmt.Errorf("get alert for detection result %d: %w", m.ID, err)
	}
	inv.AlertID = alert.ID
	inv.AlertSummary = alert.Summary
	inv.AlertAvailable = true

	chain, err := traceability.VerifyAlert(ctx, tx, alert.ID)
	if err != nil {
		return Inventory{}, fmt.Errorf("verify traceability chain for alert %d: %w", alert.ID, err)
	}
	inv.Chain = chain

	return inv, nil
}

// ErrNotFound is returned by Compose when alertID does not resolve to an
// existing alert -- the one gap Compose treats as absence of the whole
// subject, not a partial result (internal/retrieval maps this to 404).
var ErrNotFound = errors.New("evidence: alert not found")

// Compose assembles the best-effort six-artifact Inventory for alertID,
// for read-only presentation by internal/retrieval (FR-030, FR-034,
// FR-035). Unlike Advance, Compose never opens a transaction, writes
// nothing, and does not fail just because one artifact is missing or
// unavailable: each of the six reads is attempted independently, and a
// missing one is recorded as unavailable in the returned Inventory rather
// than aborting composition -- so a genuine gap is reported visibly
// (FR-035, UC-003's "insufficient or unavailable" alternative outcome)
// instead of being hidden behind an error.
//
// Compose returns ErrNotFound only when alertID itself does not resolve
// (internal/traceability.Locate finds no such alert) -- the one case
// there is no subject to compose a partial Inventory for. Any other
// non-nil error reflects a genuine read failure (context cancellation,
// database connectivity, or similar), never a missing downstream
// artifact.
func Compose(ctx context.Context, db *sql.DB, alertID int64) (*Inventory, error) {
	loc, err := traceability.Locate(ctx, db, alertID)
	if errors.Is(err, traceability.ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("evidence: compose alert %d: %w", alertID, err)
	}

	inv := &Inventory{AlertID: alertID}

	sub, err := submission.Get(ctx, db, loc.SubmissionID)
	switch {
	case errors.Is(err, submission.ErrNotFound):
	case err != nil:
		return nil, fmt.Errorf("evidence: compose alert %d: get submission: %w", alertID, err)
	default:
		inv.RawEvent = sub.RawEvent
		inv.SourceEventAvailable = len(sub.RawEvent) > 0
	}

	vr, err := validation.Get(ctx, db, loc.SubmissionID)
	switch {
	case errors.Is(err, validation.ErrNotFound):
	case err != nil:
		return nil, fmt.Errorf("evidence: compose alert %d: get validation outcome: %w", alertID, err)
	default:
		inv.ValidationOutcome = vr.Result
		inv.ValidationOutcomeAvailable = true
	}

	rec, err := normalization.Get(ctx, db, loc.SubmissionID)
	switch {
	case errors.Is(err, normalization.ErrNotFound):
	case err != nil:
		return nil, fmt.Errorf("evidence: compose alert %d: get normalized event: %w", alertID, err)
	default:
		inv.NormalizedEvent = rec.Event
		inv.NormalizedEventAvailable = true
	}

	result, err := detection.GetResult(ctx, db, loc.DetectionResultID)
	switch {
	case errors.Is(err, detection.ErrResultNotFound):
	case err != nil:
		return nil, fmt.Errorf("evidence: compose alert %d: get detection result: %w", alertID, err)
	default:
		inv.MatchReason = result.MatchReason
		inv.DetectionResultAvailable = true

		def, revision, err := detection.GetDefinition(ctx, db, result.DetectionDefinitionID)
		switch {
		case errors.Is(err, detection.ErrDefinitionNotFound):
		case err != nil:
			return nil, fmt.Errorf("evidence: compose alert %d: get detection definition: %w", alertID, err)
		default:
			inv.Definition = def
			inv.DefinitionRevision = revision
			inv.DefinitionAvailable = true
		}
	}

	alert, err := alerting.Get(ctx, db, loc.DetectionResultID)
	switch {
	case errors.Is(err, alerting.ErrNotFound):
	case err != nil:
		return nil, fmt.Errorf("evidence: compose alert %d: get alert: %w", alertID, err)
	default:
		inv.AlertSummary = alert.Summary
		inv.AlertAvailable = true
	}

	chain, err := traceability.VerifyAlert(ctx, db, alertID)
	if err != nil {
		return nil, fmt.Errorf("evidence: compose alert %d: verify traceability: %w", alertID, err)
	}
	inv.Chain = chain

	return inv, nil
}

// SummaryItem is the lightweight per-alert projection ComposeList serves
// to internal/retrieval's alert-inventory list (FR-030's list-level
// analog), deliberately never the full six-artifact Inventory Compose
// produces: an alert's persisted Summary (FR-029) already carries every
// field the inventory needs (detection name, subject, operation, target,
// outcome, request time), so a list of many alerts never has to carry
// every row's raw event, normalized event, or detection definition
// content.
type SummaryItem struct {
	AlertID int64
	Summary alerting.Summary
	Chain   traceability.Result
}

// ComposeList assembles the best-effort summary projection for every
// persisted alert, ordered by alert id ascending (alerting.List) for a
// deterministic presentation. Like Compose, a verification gap is
// reported visibly (Chain.Intact == false) rather than by omitting the
// alert or raising an error (FR-035) -- a returned error reflects only a
// genuine read failure (context cancellation, database connectivity),
// never a per-alert traceability outcome.
func ComposeList(ctx context.Context, db *sql.DB) ([]SummaryItem, error) {
	alerts, err := alerting.List(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("evidence: compose list: %w", err)
	}

	items := make([]SummaryItem, 0, len(alerts))
	for _, a := range alerts {
		chain, err := traceability.VerifyAlert(ctx, db, a.ID)
		if err != nil {
			return nil, fmt.Errorf("evidence: compose list: verify traceability chain for alert %d: %w", a.ID, err)
		}
		items = append(items, SummaryItem{AlertID: a.ID, Summary: a.Summary, Chain: chain})
	}
	return items, nil
}
