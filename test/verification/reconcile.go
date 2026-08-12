package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Every function in this file issues SELECT statements only, against the
// harness's dedicated read-only role (dbaccess.go). None of them is ever
// called with a connection other than the one SetupReadOnlyAccess
// returns. Product-state creation happens exclusively through
// APIClient.PostAuditEvent (httpclient.go), the real HTTP intake path.

// AlertCorrelation is one submission's resolved intake-to-alert timing,
// via the same FK chain internal/evidence and internal/traceability
// already walk (alert -> detection_result(matched) -> normalized_event ->
// submission). AdmittedAt is the database-authoritative admission moment
// (NFR-001's "measured from admission into the defined intake"); AlertAt,
// when non-nil, is the moment the alert became durably recorded --
// AC-020's "alert-availability time".
type AlertCorrelation struct {
	SubmissionID int64
	AdmittedAt   time.Time
	AlertID      *int64
	AlertAt      *time.Time
}

// CorrelateAlerts resolves every id in submissionIDs to its admission
// time and, if one exists yet, its alert id and alert time, in one
// batched query regardless of how many ids are requested. A submission
// with no matching alert yet (still processing, or validly non-matching)
// is present in the result with AlertID == nil, never absent -- callers
// must not treat "absent from the map" as a valid outcome; every
// requested id that exists in submissions is represented.
func CorrelateAlerts(ctx context.Context, db *sql.DB, submissionIDs []int64) (map[int64]AlertCorrelation, error) {
	out := make(map[int64]AlertCorrelation, len(submissionIDs))
	if len(submissionIDs) == 0 {
		return out, nil
	}

	rows, err := db.QueryContext(ctx, `
		SELECT s.id, s.created_at
		FROM submissions s
		WHERE s.id = ANY($1)`, submissionIDs)
	if err != nil {
		return nil, fmt.Errorf("reconcile: correlate alerts (base): %w", err)
	}
	for rows.Next() {
		var c AlertCorrelation
		if err := rows.Scan(&c.SubmissionID, &c.AdmittedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("reconcile: correlate alerts (base): scan: %w", err)
		}
		out[c.SubmissionID] = c
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("reconcile: correlate alerts (base): %w", err)
	}
	rows.Close()

	alertRows, err := db.QueryContext(ctx, `
		SELECT s.id, a.id, a.created_at
		FROM submissions s
		JOIN normalized_events ne ON ne.submission_id = s.id
		JOIN detection_results dr ON dr.normalized_event_id = ne.id AND dr.matched = true
		JOIN alerts a ON a.detection_result_id = dr.id
		WHERE s.id = ANY($1)`, submissionIDs)
	if err != nil {
		return nil, fmt.Errorf("reconcile: correlate alerts (alert join): %w", err)
	}
	defer alertRows.Close()
	for alertRows.Next() {
		var (
			subID   int64
			alertID int64
			alertAt time.Time
		)
		if err := alertRows.Scan(&subID, &alertID, &alertAt); err != nil {
			return nil, fmt.Errorf("reconcile: correlate alerts (alert join): scan: %w", err)
		}
		c := out[subID]
		if c.AlertID == nil { // keep the first row if a submission ever matched >1 definition
			id := alertID
			at := alertAt
			c.AlertID = &id
			c.AlertAt = &at
			out[subID] = c
		}
	}
	if err := alertRows.Err(); err != nil {
		return nil, fmt.Errorf("reconcile: correlate alerts (alert join): %w", err)
	}
	return out, nil
}

// StatusCounts groups every submission row by its current status --
// the backlog/queue-depth snapshot sustained_run.go samples periodically
// to build the drain time series (item 4 of the design: "backlog
// behavior").
func StatusCounts(ctx context.Context, db *sql.DB) (map[string]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT status, count(*) FROM submissions GROUP BY status`)
	if err != nil {
		return nil, fmt.Errorf("reconcile: status counts: %w", err)
	}
	defer rows.Close()

	out := make(map[string]int64, 6)
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			return nil, fmt.Errorf("reconcile: status counts: scan: %w", err)
		}
		out[status] = n
	}
	return out, rows.Err()
}

// SubmissionRowCount is the total number of submissions ever admitted --
// the no-silent-loss baseline: this count must never decrease, and every
// id the harness itself admitted must remain present (ExistingSubmissionIDs).
func SubmissionRowCount(ctx context.Context, db *sql.DB) (int64, error) {
	var n int64
	err := db.QueryRowContext(ctx, `SELECT count(*) FROM submissions`).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("reconcile: submission row count: %w", err)
	}
	return n, nil
}

// SubmissionStatuses returns the current status of every id in ids that
// still exists (an id absent from the result -- which should never
// happen, since no delete path exists anywhere in this codebase -- is a
// genuine loss finding the caller must report, not silently ignore).
func SubmissionStatuses(ctx context.Context, db *sql.DB, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := db.QueryContext(ctx, `SELECT id, status FROM submissions WHERE id = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("reconcile: submission statuses: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var status string
		if err := rows.Scan(&id, &status); err != nil {
			return nil, fmt.Errorf("reconcile: submission statuses: scan: %w", err)
		}
		out[id] = status
	}
	return out, rows.Err()
}

// DuplicateFinding is a violation of a per-artifact uniqueness constraint
// already declared in migrations/0001_initial_schema.up.sql
// (validation_outcomes(submission_id), normalized_events(submission_id),
// detection_results(normalized_event_id, detection_definition_id),
// alerts(detection_result_id)). This check should always return an empty
// slice -- Postgres itself enforces these constraints -- so it exists as
// an independent, harness-level confirmation of a DB-level guarantee
// (item 4/5 of the design), not a mechanism the harness relies on to
// prevent duplicates itself.
type DuplicateFinding struct {
	Table string
	Key   string
	Count int64
}

var duplicateChecks = []struct {
	table string
	query string
}{
	{"validation_outcomes", `SELECT submission_id::text, count(*) FROM validation_outcomes GROUP BY submission_id HAVING count(*) > 1`},
	{"normalized_events", `SELECT submission_id::text, count(*) FROM normalized_events GROUP BY submission_id HAVING count(*) > 1`},
	{"detection_results", `SELECT normalized_event_id || ',' || detection_definition_id, count(*) FROM detection_results GROUP BY normalized_event_id, detection_definition_id HAVING count(*) > 1`},
	{"alerts", `SELECT detection_result_id::text, count(*) FROM alerts GROUP BY detection_result_id HAVING count(*) > 1`},
}

func CheckNoDuplicates(ctx context.Context, db *sql.DB) ([]DuplicateFinding, error) {
	var findings []DuplicateFinding
	for _, chk := range duplicateChecks {
		rows, err := db.QueryContext(ctx, chk.query)
		if err != nil {
			return nil, fmt.Errorf("reconcile: duplicate check (%s): %w", chk.table, err)
		}
		for rows.Next() {
			var key string
			var count int64
			if err := rows.Scan(&key, &count); err != nil {
				rows.Close()
				return nil, fmt.Errorf("reconcile: duplicate check (%s): scan: %w", chk.table, err)
			}
			findings = append(findings, DuplicateFinding{Table: chk.table, Key: key, Count: count})
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return nil, fmt.Errorf("reconcile: duplicate check (%s): %w", chk.table, err)
		}
		rows.Close()
	}
	return findings, nil
}
