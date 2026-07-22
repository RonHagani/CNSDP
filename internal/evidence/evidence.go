// Package evidence implements the evidence inventory module (ARCH-01 §2
// module 6): FR-031 through FR-035's per-alert account of the six
// minimum-evidence-set artifacts, verified complete before a submission
// may advance from alerted to evidenced. It owns no artifact table of its
// own -- exactly like internal/traceability, every artifact it reads
// comes through the owning module's sanctioned read API
// (internal/submission, internal/validation, internal/normalization,
// internal/detection, internal/alerting, internal/traceability), never by
// querying another module's table directly.
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
