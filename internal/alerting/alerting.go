// Package alerting implements the alerting module (ARCH-01 §2 module 5):
// FR-027 through FR-030's production of exactly one explainable alert per
// matching detection result, and FR-029's required alert content. It owns
// every read and write against alerts (ADR-0002); no other package writes
// this table. The matched detection results and normalized event it needs
// are read exclusively through internal/detection.MatchedResults and
// internal/normalization.Get -- the sanctioned read paths those packages
// export -- never through direct SQL against their tables.
package alerting

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cnsdp/internal/detection"
	"cnsdp/internal/normalization"
	"cnsdp/internal/submission"
)

// Summary is the persisted content of an alerts row (FR-029): the matched
// detection definition's identity and satisfied conditions, taken
// verbatim from the detection_results row that produced this alert --
// never re-resolved from the currently active definition set, which
// could disagree with an older persisted result after a definition
// change (AC-014) -- plus the normalized event content identifying what
// was detected: the requesting subject, the operation, the target
// resource, the recorded outcome, and the request time.
type Summary struct {
	MatchReason detection.MatchReason   `json:"matchReason"`
	Subject     normalization.Subject   `json:"subject"`
	Operation   normalization.Operation `json:"operation"`
	Target      normalization.Target    `json:"target"`
	Outcome     normalization.Outcome   `json:"outcome"`
	RequestTime time.Time               `json:"requestTime"`
}

// buildSummary assembles one alert's Summary from a matched detection
// result and the normalized event it was evaluated against. Pulled out
// as its own pure function so alert content can be tested without a
// database.
func buildSummary(mr detection.MatchReason, event normalization.Event) Summary {
	return Summary{
		MatchReason: mr,
		Subject:     event.Subject,
		Operation:   event.Operation,
		Target:      event.Target,
		Outcome:     event.Outcome,
		RequestTime: event.RequestTime,
	}
}

// ErrSummaryConflict is returned by Advance when an alerts row already
// exists for a detection result with a different summary than what this
// call just computed -- a genuine disagreement between the newly computed
// summary and the one already on record. It is never silently resolved by
// overwriting the previously persisted artifact (NFR-006, NFR-017): the
// submission is left at evaluated and the caller must investigate.
var ErrSummaryConflict = errors.New("alerting: existing alert has different summary")

// Advance generates one alert for every matching detection result
// recorded for sub's normalized event (FR-027, FR-028, FR-029) and
// advances the submission from evaluated to alerted -- all in one
// transaction (ADR-0002). internal/alerting owns every read and write
// against alerts; the worker only selects and dispatches work, and never
// writes this table itself.
//
// The evaluated -> alerted transition is unconditional: a submission with
// zero matching detection results produces zero alerts but still
// advances, because this status marks completed alert-generation
// processing, not alert existence (AC-013 branch (a)).
//
// Retry safety: a repeated call for the same submission_id is accepted as
// a safe no-op retry only when, for every matching detection result, the
// newly computed summary exactly matches what is already persisted --
// detected by one race-safe INSERT ... ON CONFLICT ... DO UPDATE ...
// WHERE statement per alert, the same idempotent-insert pattern
// internal/detection.Advance and the other stages already use. A
// mismatch on any one alert returns ErrSummaryConflict and aborts the
// whole transaction before any status change; no row is ever silently
// overwritten.
func Advance(ctx context.Context, db *sql.DB, sub *submission.Submission) error {
	rec, err := normalization.Get(ctx, db, sub.ID)
	if err != nil {
		return fmt.Errorf("alerting: advance submission %d: %w", sub.ID, err)
	}

	matches, err := detection.MatchedResults(ctx, db, rec.ID)
	if err != nil {
		return fmt.Errorf("alerting: advance submission %d: %w", sub.ID, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("alerting: begin advance tx for submission %d: %w", sub.ID, err)
	}
	defer tx.Rollback()

	for _, m := range matches {
		summary, err := json.Marshal(buildSummary(m.MatchReason, rec.Event))
		if err != nil {
			return fmt.Errorf("alerting: advance submission %d: marshal summary for detection result %d: %w", sub.ID, m.ID, err)
		}
		if err := upsertAlert(ctx, tx, m.ID, summary); err != nil {
			return fmt.Errorf("alerting: advance submission %d: %w", sub.ID, err)
		}
	}

	if err := submission.Advance(ctx, tx, sub.ID, submission.StatusEvaluated, submission.StatusAlerted); err != nil {
		return fmt.Errorf("alerting: advance submission %d to alerted: %w", sub.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("alerting: commit advance for submission %d: %w", sub.ID, err)
	}
	return nil
}

// upsertAlert performs one race-safe, conflict-aware insert of a single
// detection result's alert.
func upsertAlert(ctx context.Context, tx *sql.Tx, detectionResultID int64, summary []byte) error {
	var id int64
	err := tx.QueryRowContext(ctx,
		`INSERT INTO alerts (detection_result_id, summary)
		 VALUES ($1, $2)
		 ON CONFLICT (detection_result_id) DO UPDATE
		 SET summary = EXCLUDED.summary
		 WHERE alerts.summary = EXCLUDED.summary
		 RETURNING id`,
		detectionResultID, summary,
	).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: detection result %d", ErrSummaryConflict, detectionResultID)
	}
	if err != nil {
		return fmt.Errorf("insert alert for detection result %d: %w", detectionResultID, err)
	}
	return nil
}

// ErrNotFound is returned by Get when no alert has been persisted for the
// given detection result id.
var ErrNotFound = errors.New("alerting: alert not found")

// Alert is one persisted alerts row: its own id -- the value
// internal/traceability's chain walk resolves through -- the detection
// result it was generated from, and its recorded Summary.
type Alert struct {
	ID                int64
	DetectionResultID int64
	Summary           Summary
}

// DB is the minimal subset of *sql.DB / *sql.Tx Get needs, so a caller
// can pass either a plain connection or an already-open transaction --
// e.g. internal/evidence's verification transaction (ADR-0002).
type DB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Get retrieves the alert already persisted for detectionResultID. It is
// the sanctioned way a downstream module reads alerts: internal/alerting
// owns every read and write against this table (see package doc), so a
// stage that needs the alert -- currently evidence assembly -- calls Get
// rather than querying the table directly. Returns ErrNotFound if no
// alert has been persisted for this detection result yet.
func Get(ctx context.Context, db DB, detectionResultID int64) (*Alert, error) {
	var (
		id  int64
		raw []byte
	)
	err := db.QueryRowContext(ctx,
		`SELECT id, summary FROM alerts WHERE detection_result_id = $1`,
		detectionResultID,
	).Scan(&id, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("alerting: get alert for detection result %d: %w", detectionResultID, err)
	}

	var summary Summary
	if err := json.Unmarshal(raw, &summary); err != nil {
		return nil, fmt.Errorf("alerting: get alert for detection result %d: unmarshal summary: %w", detectionResultID, err)
	}
	return &Alert{ID: id, DetectionResultID: detectionResultID, Summary: summary}, nil
}
