// Package traceability implements the traceability module (ARCH-01 §2
// module 7): FR-033 and FR-034's alert-to-source traceability chain, and
// NFR-029's requirement that the chain be automatically, independently
// verifiable, failing visibly and identifying the specific affected link
// when it does not resolve intact (AC-016).
//
// This module owns no artifact table of its own -- there is no
// traceability_links table (ARCH-01 §4): the chain is fully derivable
// through detection_results.normalized_event_id and
// normalized_events.submission_id, both NOT NULL foreign keys. ARCH-01 §4
// grants this module specifically the one exception to "no module reads
// another module's table directly": it derives and exposes the chain by
// joining across alerts, detection_results, normalized_events, and
// submissions directly, rather than composing each owning module's
// individual sanctioned reads.
package traceability

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"

	"cnsdp/internal/submission"
)

// Result is the outcome of verifying one alert's traceability chain: Chain
// existence is structurally guaranteed by the FK-enforced schema under
// ordinary application operation (ARCH-01 §4), so the two out-of-band
// tamper-evidence checks this function performs -- source-key
// re-derivation and raw-event digest comparison -- are what can actually
// fail in practice; a nonexistent alert id is the one reachable
// structural failure. When Intact is false, FailedLink names the specific
// affected relationship (AC-016): "alert" (chain does not resolve),
// "source_key" (audit-identity drift), or "raw_event_sha256" (raw-payload
// modification).
type Result struct {
	Intact     bool
	FailedLink string
}

// DB is the minimal subset of *sql.DB / *sql.Tx VerifyAlert needs, so a
// caller can pass either a plain connection or an already-open
// transaction -- e.g. internal/evidence's verification transaction
// (ADR-0002).
type DB interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// VerifyAlert walks the traceability chain for alertID -- alerts ->
// detection_results -> normalized_events -> submissions -- in one join,
// and additionally checks the two out-of-band tamper-evidence signals the
// FK-enforced schema cannot cover on its own (NFR-017, AC-015):
//
//   - re-deriving the submission's source_key from its currently stored
//     raw_event/audit_id/audit_stage (submission.SourceKey) and comparing
//     it to the persisted source_key column, detecting audit-identity
//     drift; and
//   - hashing the currently stored raw_event and comparing it to the
//     persisted raw_event_sha256 (migrations/0003_submission_integrity),
//     detecting raw-payload modification.
//
// Scope: this detects a raw event or identity field changed without the
// corresponding integrity metadata being changed alongside it. It does
// not defend against a privileged database writer who coherently
// replaces both the content and its digest/key together -- that would
// require an external trust anchor, signature, or keyed MAC, which is out
// of scope for this checkpoint (no NFR-017 or AC-015 wording requires
// it).
//
// A business-level failure (missing chain or tamper detected) is reported
// as Result{Intact: false, FailedLink: ...}, never a Go error; error is
// reserved for genuine read failures (context cancellation,
// connectivity), so this fails visibly rather than indistinguishably from
// an operational fault (AC-016).
func VerifyAlert(ctx context.Context, db DB, alertID int64) (Result, error) {
	var (
		auditID, auditStage, family, sourceEventID, storedSourceKey string
		rawEvent, storedDigest                                      []byte
	)
	err := db.QueryRowContext(ctx,
		`SELECT s.audit_id, s.audit_stage, s.source_family, s.source_event_id, s.raw_event, s.source_key, s.raw_event_sha256
		 FROM alerts a
		 JOIN detection_results r ON r.id = a.detection_result_id
		 JOIN normalized_events n ON n.id = r.normalized_event_id
		 JOIN submissions s ON s.id = n.submission_id
		 WHERE a.id = $1`,
		alertID,
	).Scan(&auditID, &auditStage, &family, &sourceEventID, &rawEvent, &storedSourceKey, &storedDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return Result{Intact: false, FailedLink: "alert"}, nil
	}
	if err != nil {
		return Result{}, fmt.Errorf("traceability: verify alert %d: %w", alertID, err)
	}

	if submission.SourceKey(rawEvent, submission.Family(family), auditID, auditStage, sourceEventID) != storedSourceKey {
		return Result{Intact: false, FailedLink: "source_key"}, nil
	}

	digest := sha256.Sum256(rawEvent)
	if !bytes.Equal(digest[:], storedDigest) {
		return Result{Intact: false, FailedLink: "raw_event_sha256"}, nil
	}

	return Result{Intact: true}, nil
}

// ErrNotFound is returned by Locate when alertID does not resolve to an
// existing alert.
var ErrNotFound = errors.New("traceability: alert not found")

// Locator resolves the ids internal/evidence needs to fetch each
// artifact's content through its owning module's own sanctioned read.
// Locate itself never reads artifact content -- only the links between
// artifacts, which is exactly the cross-table join exception ARCH-01 §4
// grants this module; content ownership and reading remain with each
// artifact's own module (internal/detection, internal/alerting, etc.).
type Locator struct {
	DetectionResultID int64
	SubmissionID      int64
}

// Locate resolves alertID's position in the traceability chain -- alerts
// -> detection_results -> normalized_events -> submissions -- in one
// join, the same cross-table read VerifyAlert performs. Returns
// ErrNotFound if alertID does not exist.
func Locate(ctx context.Context, db DB, alertID int64) (Locator, error) {
	var loc Locator
	err := db.QueryRowContext(ctx,
		`SELECT r.id, s.id
		 FROM alerts a
		 JOIN detection_results r ON r.id = a.detection_result_id
		 JOIN normalized_events n ON n.id = r.normalized_event_id
		 JOIN submissions s ON s.id = n.submission_id
		 WHERE a.id = $1`,
		alertID,
	).Scan(&loc.DetectionResultID, &loc.SubmissionID)
	if errors.Is(err, sql.ErrNoRows) {
		return Locator{}, ErrNotFound
	}
	if err != nil {
		return Locator{}, fmt.Errorf("traceability: locate alert %d: %w", alertID, err)
	}
	return loc, nil
}
