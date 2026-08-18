// Package worker orchestrates the single-worker durable processing loop
// (ADR-0002): select the oldest eligible submission, dispatch it to
// exactly the one stage module that owns its current status, repeat. It
// owns no table itself and contains no stage business logic of its own —
// dispatch delegates entirely to each stage module's own Advance
// function (internal/validation, internal/normalization,
// internal/detection, internal/alerting, internal/evidence), each of
// which owns every read and write against its own artifact table (or, for
// internal/evidence, no table of its own -- see its package doc). The
// read-only join in oldestEligible is an orchestration-only exception to
// "no module reads another module's table directly": it never writes any
// table and contains no stage business logic, only the claim predicate
// needed to select work. All nine ARCH-01 §2 workflow modules are
// implemented; worker is the orchestration layer that drives submissions
// through them, not a stage module itself.
//
// Run (Checkpoint 11) is the continuous loop cmd/platform starts: it
// calls ProcessOne's underlying claim-and-dispatch repeatedly until ctx is
// canceled. A submission that fails one stage attempt must not monopolize
// every later iteration (NFR-008) — Run tracks a purely in-memory,
// non-persistent sweep cursor over the existing created_at/id eligibility
// ordering. Once a failure opens a sweep, the cursor advances past every
// submission it attempts next, whether that attempt succeeds or fails, so
// each later eligible submission gets exactly one attempt before the
// failed one is reconsidered. The sweep ends, and the cursor resets, only
// when no eligible submission remains after it; a fresh sweep then starts
// from the global oldest again after one pollInterval wait. Outside a
// sweep (the common, no-failure case) the cursor stays nil and Run
// behaves exactly like repeated ProcessOne calls, preserving the existing
// age-fairness behavior and its tests unchanged.
package worker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"cnsdp/internal/alerting"
	"cnsdp/internal/db"
	"cnsdp/internal/detection"
	"cnsdp/internal/evidence"
	"cnsdp/internal/normalization"
	"cnsdp/internal/submission"
	"cnsdp/internal/validation"
)

// ErrNoWork is returned by ProcessOne when there is no submission at a
// status this worker knows how to advance.
var ErrNoWork = submission.ErrNoWork

// ProcessOne selects the oldest eligible submission across every stage
// this worker knows how to advance and dispatches it to exactly one
// stage. No SKIP LOCKED, no concurrency: this function assumes it is the
// only writer running against the database (ADR-0002) — a future need for
// concurrent workers requires its own claim-locking design, not an
// extension of this one.
func ProcessOne(ctx context.Context, db *sql.DB) error {
	sub, err := oldestEligible(ctx, db)
	if err != nil {
		if errors.Is(err, submission.ErrNoWork) {
			return ErrNoWork
		}
		return fmt.Errorf("worker: claim next eligible submission: %w", err)
	}
	return dispatch(ctx, db, sub)
}

// dispatch sends sub to exactly the one stage module that owns its
// current status. Extracted from ProcessOne so Run can reuse the same
// dispatch behavior without duplicating it.
func dispatch(ctx context.Context, db *sql.DB, sub *submission.Submission) error {
	switch sub.Status {
	case submission.StatusAdmitted:
		return validation.Advance(ctx, db, sub)
	case submission.StatusValidated:
		return normalization.Advance(ctx, db, sub)
	case submission.StatusNormalized:
		return detection.Advance(ctx, db, sub)
	case submission.StatusEvaluated:
		return alerting.Advance(ctx, db, sub)
	case submission.StatusAlerted:
		return evidence.Advance(ctx, db, sub)
	default:
		return fmt.Errorf("worker: claimed submission %d has unexpected status %q", sub.ID, sub.Status)
	}
}

// cursor marks a sweep position in the existing (created_at, id)
// eligibility ordering. It exists only in Run's stack frame -- never
// persisted, never a durable retry mechanism.
type cursor struct {
	createdAt time.Time
	id        int64
}

// Run continuously claims and dispatches eligible submissions until ctx
// is canceled, applying the sweep-cursor fairness rule described in the
// package doc. It never returns an error: a claim or dispatch failure is
// logged (NFR-021, NFR-022) and processing continues.
func Run(ctx context.Context, db *sql.DB, pollInterval time.Duration) {
	var cur *cursor
	for {
		if ctx.Err() != nil {
			return
		}

		sub, err := oldestEligibleAfter(ctx, db, cur)
		if errors.Is(err, submission.ErrNoWork) {
			cur = nil
			waitOrDone(ctx, pollInterval)
			continue
		}
		if err != nil {
			slog.Error("worker: claim next eligible submission failed", "error_family", errorFamily(err))
			cur = nil
			waitOrDone(ctx, pollInterval)
			continue
		}

		sweeping := cur != nil

		if dispatchErr := dispatch(ctx, db, sub); dispatchErr != nil {
			slog.Error("worker: stage attempt failed",
				"submission_id", sub.ID,
				"stage", string(sub.Status),
				"error_family", errorFamily(dispatchErr),
			)
			cur = &cursor{sub.CreatedAt, sub.ID}
			continue
		}
		slog.Debug("worker: stage attempt succeeded", "submission_id", sub.ID, "stage", string(sub.Status))

		if sweeping {
			cur = &cursor{sub.CreatedAt, sub.ID}
		} else {
			cur = nil
		}
	}
}

// waitOrDone blocks for d, or until ctx is canceled, whichever comes
// first -- so a canceled Run never waits out a full pollInterval before
// returning.
func waitOrDone(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// errorFamily classifies a claim or dispatch error into a coarse,
// content-free category for structured logging (NFR-015, NFR-021): never
// the error's full message text, which could in principle echo
// driver-level detail, and never any source-telemetry content.
func errorFamily(err error) string {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "context_canceled"
	case errors.Is(err, validation.ErrOutcomeConflict),
		errors.Is(err, normalization.ErrArtifactConflict),
		errors.Is(err, detection.ErrResultConflict),
		errors.Is(err, alerting.ErrSummaryConflict):
		return "artifact_conflict"
	case errors.Is(err, submission.ErrStatusConflict):
		return "status_conflict"
	case errors.Is(err, submission.ErrNotFound):
		return "not_found"
	case db.IsResourceExhausted(err):
		// A stage's artifact insert or status advance hit the documented
		// persistent-storage resource limit (NFR-036; AC-023) -- a
		// platform-fault outcome, distinct from every processing_error
		// above it: none of those represent the platform itself running
		// out of a resource it depends on.
		return "resource_exhausted"
	default:
		return "processing_error"
	}
}

// oldestEligible selects the single oldest submission this worker can
// currently advance, across five eligibility conditions:
//
//   - status = admitted (every admitted submission needs validation),
//   - status = validated with a recorded valid outcome (FR-014: only a
//     valid submission proceeds to normalization; invalid, incomplete, and
//     unsupported submissions are permanently parked at validated with no
//     handler that ever claims them again),
//   - status = normalized (every normalized submission needs detection
//     evaluation; no outcome gate applies here since normalization
//     unconditionally produces one normalized event per valid submission),
//   - status = evaluated (every evaluated submission needs alert
//     generation; no outcome gate applies here either -- the
//     evaluated -> alerted transition is unconditional, since a
//     submission with zero matching detection results still completes
//     alert-generation processing, just with zero alerts produced), or
//   - status = alerted (every alerted submission needs evidence
//     verification; no outcome gate applies here either -- the
//     alerted -> evidenced transition is unconditional, since a
//     submission with zero alerts has an empty evidence-inventory set,
//     which still verifies vacuously as a successfully completed stage).
//
// Ordering is by submission creation order (created_at), with id as the
// deterministic tie-breaker for rows created in the same instant. No
// stage is given blanket priority over another: under sustained intake,
// an "admitted always wins" policy would starve eligible validated,
// normalized, evaluated, or alerted submissions indefinitely, so
// eligibility is decided by age across all five conditions together, not
// by stage.
//
// This is a read-only join against validation_outcomes -- a table
// internal/validation owns -- solely to evaluate the claim predicate
// above; it never writes any table and contains no stage business logic
// beyond referencing the outcome vocabulary internal/validation already
// exports.
func oldestEligible(ctx context.Context, db *sql.DB) (*submission.Submission, error) {
	return oldestEligibleAfter(ctx, db, nil)
}

// oldestEligibleAfter is oldestEligible restricted, when after is
// non-nil, to submissions strictly past after in the same (created_at,
// id) ordering ORDER BY already uses -- the read-only mechanism Run's
// sweep cursor relies on to skip a just-failed submission without
// touching any table. after == nil reproduces oldestEligible's original,
// unrestricted query exactly.
func oldestEligibleAfter(ctx context.Context, db *sql.DB, after *cursor) (*submission.Submission, error) {
	query := `SELECT s.id, s.status, s.raw_event, s.audit_id, s.audit_stage, s.source_family, s.source_event_id, s.created_at
		 FROM submissions s
		 LEFT JOIN validation_outcomes v ON v.submission_id = s.id
		 WHERE (s.status = $1
		    OR (s.status = $2 AND v.outcome = $3)
		    OR s.status = $4
		    OR s.status = $5
		    OR s.status = $6)`
	args := []any{
		string(submission.StatusAdmitted), string(submission.StatusValidated), string(validation.OutcomeValid),
		string(submission.StatusNormalized), string(submission.StatusEvaluated), string(submission.StatusAlerted),
	}
	if after != nil {
		query += ` AND (s.created_at, s.id) > ($7, $8)`
		args = append(args, after.createdAt, after.id)
	}
	query += ` ORDER BY s.created_at, s.id LIMIT 1`

	row := db.QueryRowContext(ctx, query, args...)

	var sub submission.Submission
	var status, family string
	if err := row.Scan(&sub.ID, &status, &sub.RawEvent, &sub.AuditID, &sub.AuditStage, &family, &sub.SourceEventID, &sub.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, submission.ErrNoWork
		}
		return nil, fmt.Errorf("worker: select oldest eligible submission: %w", err)
	}
	sub.Status = submission.Status(status)
	sub.SourceFamily = submission.Family(family)
	return &sub, nil
}
