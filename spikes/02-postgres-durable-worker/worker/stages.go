package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
)

// CrashSpec identifies exactly one (submission, stage, timing) point at
// which this process should terminate abruptly (os.Exit) instead of
// continuing normally. Only ever one target per invocation — this spike
// tests one deterministic crash point per run, not random/fuzzed timing.
type CrashSpec struct {
	SubmissionID int
	Stage        string
	Timing       string // "before", "mid", "after"
}

func (c CrashSpec) matches(submissionID int, stage, timing string) bool {
	return c.SubmissionID != 0 && c.SubmissionID == submissionID &&
		c.Stage == stage && c.Timing == timing
}

// crashExitCode is deliberately distinctive so the driver can confirm an
// injected crash actually happened, rather than the worker finishing
// normally without ever reaching the intended point.
const crashExitCode = 111

func crashNow(reason string) {
	fmt.Fprintf(os.Stderr, "SPIKE-CRASH-INJECTED: %s\n", reason)
	os.Exit(crashExitCode)
}

type stmt struct {
	sql  string
	args []interface{}
}

// stageWork describes one durable state transition: the submission's
// required prior status, its resulting status, and the artifact insert(s)
// that must land in the SAME transaction as the status update. prepare
// runs inside that transaction so it can read state written by earlier
// stages (e.g. the alert stage needs the detection_result id produced by
// the evaluate stage).
type stageWork struct {
	name       string
	fromStatus string
	toStatus   string
	prepare    func(ctx context.Context, tx *sql.Tx, submissionID int) ([]stmt, error)
}

var stages = []stageWork{
	{
		name:       "validate",
		fromStatus: "admitted",
		toStatus:   "validated",
		prepare: func(_ context.Context, _ *sql.Tx, submissionID int) ([]stmt, error) {
			return []stmt{{
				sql:  `INSERT INTO validation_outcomes (submission_id, outcome) VALUES ($1, 'valid') ON CONFLICT (submission_id) DO NOTHING`,
				args: []interface{}{submissionID},
			}}, nil
		},
	},
	{
		name:       "normalize",
		fromStatus: "validated",
		toStatus:   "normalized",
		prepare: func(_ context.Context, _ *sql.Tx, submissionID int) ([]stmt, error) {
			return []stmt{{
				sql:  `INSERT INTO normalized_events (submission_id, content) VALUES ($1, 'normalized-stub') ON CONFLICT (submission_id) DO NOTHING`,
				args: []interface{}{submissionID},
			}}, nil
		},
	},
	{
		// Evaluates against every stub detection definition in one stage,
		// mirroring FR-023 ("every normalized event individually against
		// the documented conditions of each of the three detection
		// definitions") — here, two stub definitions, both written in the
		// same transaction as the status advance.
		name:       "evaluate",
		fromStatus: "normalized",
		toStatus:   "evaluated",
		prepare: func(ctx context.Context, tx *sql.Tx, submissionID int) ([]stmt, error) {
			rows, err := tx.QueryContext(ctx, `SELECT id, name FROM detection_definitions ORDER BY id`)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			var out []stmt
			for rows.Next() {
				var id int
				var name string
				if err := rows.Scan(&id, &name); err != nil {
					return nil, err
				}
				matched := name == "scenario-stub-a" // deterministic: exactly one definition matches
				out = append(out, stmt{
					sql:  `INSERT INTO detection_results (submission_id, detection_definition_id, matched) VALUES ($1, $2, $3) ON CONFLICT (submission_id, detection_definition_id) DO NOTHING`,
					args: []interface{}{submissionID, id, matched},
				})
			}
			return out, rows.Err()
		},
	},
	{
		name:       "alert",
		fromStatus: "evaluated",
		toStatus:   "alerted",
		prepare: func(ctx context.Context, tx *sql.Tx, submissionID int) ([]stmt, error) {
			var detectionResultID int
			err := tx.QueryRowContext(ctx,
				`SELECT id FROM detection_results WHERE submission_id=$1 AND matched=true`,
				submissionID).Scan(&detectionResultID)
			if err != nil {
				return nil, fmt.Errorf("locate matching detection result: %w", err)
			}
			return []stmt{{
				sql:  `INSERT INTO alerts (detection_result_id) VALUES ($1) ON CONFLICT (detection_result_id) DO NOTHING`,
				args: []interface{}{detectionResultID},
			}}, nil
		},
	},
	{
		name:       "evidence",
		fromStatus: "alerted",
		toStatus:   "evidenced",
		prepare: func(ctx context.Context, tx *sql.Tx, submissionID int) ([]stmt, error) {
			var alertID, detectionResultID, normalizedEventID int
			err := tx.QueryRowContext(ctx, `
				SELECT a.id, a.detection_result_id, n.id
				FROM alerts a
				JOIN detection_results dr ON dr.id = a.detection_result_id
				JOIN normalized_events n ON n.submission_id = dr.submission_id
				WHERE dr.submission_id = $1 AND dr.matched = true`,
				submissionID).Scan(&alertID, &detectionResultID, &normalizedEventID)
			if err != nil {
				return nil, fmt.Errorf("locate evidence artifacts: %w", err)
			}
			sub := fmt.Sprintf("submission:%d", submissionID)
			norm := fmt.Sprintf("normalized_event:%d", normalizedEventID)
			det := fmt.Sprintf("detection_result:%d", detectionResultID)
			al := fmt.Sprintf("alert:%d", alertID)
			links := []struct{ src, tgt, rel string }{
				{al, det, "explained_by"},
				{al, norm, "evaluated"},
				{norm, sub, "derived_from"},
			}
			out := make([]stmt, 0, len(links))
			for _, l := range links {
				out = append(out, stmt{
					sql:  `INSERT INTO traceability_links (source_ref, target_ref, relation) VALUES ($1, $2, $3) ON CONFLICT (source_ref, target_ref, relation) DO NOTHING`,
					args: []interface{}{l.src, l.tgt, l.rel},
				})
			}
			return out, nil
		},
	},
}

func stageIndexForStatus(status string) int {
	for i, s := range stages {
		if s.fromStatus == status {
			return i
		}
	}
	return -1
}

// advance performs exactly one stage transition for one submission. The
// artifact insert(s) and the status UPDATE happen in the same transaction,
// per the approved refinement: no split artifact/status commits.
func advance(ctx context.Context, db *sql.DB, submissionID int, work stageWork, crash CrashSpec) error {
	if crash.matches(submissionID, work.name, "before") {
		crashNow(fmt.Sprintf("before stage %q tx begins (submission %d)", work.name, submissionID))
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmts, err := work.prepare(ctx, tx, submissionID)
	if err != nil {
		return fmt.Errorf("prepare stage %q: %w", work.name, err)
	}
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s.sql, s.args...); err != nil {
			return fmt.Errorf("exec stage %q insert: %w", work.name, err)
		}
	}

	res, err := tx.ExecContext(ctx,
		`UPDATE submissions SET status=$1 WHERE id=$2 AND status=$3`,
		work.toStatus, submissionID, work.fromStatus)
	if err != nil {
		return fmt.Errorf("update status stage %q: %w", work.name, err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("stage %q: expected to advance 1 row from %q, affected %d",
			work.name, work.fromStatus, n)
	}

	if crash.matches(submissionID, work.name, "mid") {
		crashNow(fmt.Sprintf("mid stage %q tx, before commit (submission %d)", work.name, submissionID))
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit stage %q: %w", work.name, err)
	}

	if crash.matches(submissionID, work.name, "after") {
		crashNow(fmt.Sprintf("after stage %q commit, before next stage (submission %d)", work.name, submissionID))
	}

	return nil
}

// runWorkerLoop is the entire "worker": a single-threaded, single-consumer
// loop over non-terminal submissions, oldest first. No SKIP LOCKED, no
// concurrency — nothing else is claiming rows at the same time.
func runWorkerLoop(ctx context.Context, db *sql.DB, crash CrashSpec) error {
	for {
		var submissionID int
		var status string
		err := db.QueryRowContext(ctx,
			`SELECT id, status FROM submissions WHERE status <> 'evidenced' ORDER BY id LIMIT 1`,
		).Scan(&submissionID, &status)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return fmt.Errorf("claim next submission: %w", err)
		}

		idx := stageIndexForStatus(status)
		if idx == -1 {
			return fmt.Errorf("submission %d has unrecognized status %q", submissionID, status)
		}

		if err := advance(ctx, db, submissionID, stages[idx], crash); err != nil {
			return fmt.Errorf("advance submission %d: %w", submissionID, err)
		}
	}
}
