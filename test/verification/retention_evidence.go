package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

// RetentionBaseline is the runtime evidence captured early in the run --
// while the sustained-run phase's own data is still fresh -- that
// VerifyRetentionLate later re-checks. Captured through the same
// mechanisms every other phase already uses (CorrelateAlerts, a direct
// read against the read-only role, the real retrieval API): no parallel
// verification machinery invented for this alone.
type RetentionBaseline struct {
	CapturedAt time.Time

	EarlySubmissionID     int64
	EarlySubmissionSHA256 []byte // submissions.raw_event_sha256 -- the schema's own tamper-evidence snapshot (migration 0003)

	EarlyAlertID       int64
	EarlyAlertResponse map[string]any // full GET /v1/alerts/{id} body -- the richest single artifact, spanning the whole traceability chain

	RowCounts map[string]int64
}

// CaptureRetentionBaseline picks the run's earliest admitted submission
// (by SentAt) and, if the sustained run produced one, its earliest
// resolved alert, and snapshots their persisted content -- never
// synthesized data, always something the real sustained-run traffic
// (sustained_run.go) actually produced.
func CaptureRetentionBaseline(ctx context.Context, db *sql.DB, api *APIClient, result *SustainedRunResult) (*RetentionBaseline, error) {
	b := &RetentionBaseline{CapturedAt: time.Now()}

	var earliestOK *Attempt
	for i := range result.Attempts {
		a := &result.Attempts[i]
		if a.OK {
			earliestOK = a
			break // Attempts is already sorted by SentAt (sustained_run.go)
		}
	}
	if earliestOK == nil {
		return nil, envErrorf("capture retention baseline", "no admitted attempt found in the sustained run -- nothing to use as an early artifact")
	}
	b.EarlySubmissionID = earliestOK.SubmissionID

	if err := db.QueryRowContext(ctx, `SELECT raw_event_sha256 FROM submissions WHERE id = $1`, b.EarlySubmissionID).Scan(&b.EarlySubmissionSHA256); err != nil {
		return nil, wrapEnvErr("capture retention baseline", fmt.Errorf("read early submission %d: %w", b.EarlySubmissionID, err))
	}

	var earliestAlertID *int64
	var earliestAlertSentAt time.Time
	for _, a := range result.Attempts {
		if !a.OK || a.Scenario == "" {
			continue
		}
		c, ok := result.Correlations[a.SubmissionID]
		if !ok || c.AlertID == nil {
			continue
		}
		if earliestAlertID == nil || a.SentAt.Before(earliestAlertSentAt) {
			earliestAlertID = c.AlertID
			earliestAlertSentAt = a.SentAt
		}
	}
	if earliestAlertID != nil {
		b.EarlyAlertID = *earliestAlertID
		status, body, err := api.GetJSON(ctx, fmt.Sprintf("/v1/alerts/%d", b.EarlyAlertID))
		if err != nil {
			return nil, wrapEnvErr("capture retention baseline", fmt.Errorf("retrieve early alert %d: %w", b.EarlyAlertID, err))
		}
		if status != 200 {
			return nil, envErrorf("capture retention baseline", "early alert %d retrieval returned status %d, want 200", b.EarlyAlertID, status)
		}
		b.EarlyAlertResponse = body
	}

	counts, err := tableRowCounts(ctx, db)
	if err != nil {
		return nil, err
	}
	b.RowCounts = counts

	return b, nil
}

// noDeleteStaticCheck greps every non-test .go file in the six
// workflow-stage packages that own the product tables (submissions,
// validation_outcomes, normalized_events, detection_results, alerts;
// normalization/detection/alerting/evidence own the remaining artifact
// tables between them) for a literal SQL DELETE statement -- supplementary
// corroboration only (per design), never the primary proof: the primary
// proof is the runtime before/after comparison in VerifyRetentionLate.
func noDeleteStaticCheck(repoRoot string) (clean bool, detail string) {
	dirs := []string{"submission", "validation", "normalization", "detection", "alerting", "evidence"}
	var findings []string
	for _, dir := range dirs {
		pkgDir := filepath.Join(repoRoot, "internal", dir)
		entries, err := os.ReadDir(pkgDir)
		if err != nil {
			return false, fmt.Sprintf("could not read %s: %v", pkgDir, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(pkgDir, name))
			if err != nil {
				return false, fmt.Sprintf("could not read %s: %v", name, err)
			}
			if strings.Contains(strings.ToUpper(string(raw)), "DELETE FROM") {
				findings = append(findings, filepath.Join(dir, name))
			}
		}
	}
	if len(findings) > 0 {
		return false, fmt.Sprintf("DELETE FROM found in: %v", findings)
	}
	return true, fmt.Sprintf("no DELETE FROM statement found across %v", dirs)
}

// VerifyRetentionLate re-checks baseline against the still-fully-intact
// primary database, strictly before the destructive capacity fixture is
// ever created (main.go's phase ordering) -- so this evidence is captured
// while nothing has had any opportunity to delete or corrupt it, exactly
// like the AC-019/AC-020/AC-021/AC-022 evidence already captured before
// it. Produces the two clauses AC-023 requires beyond bounded_growth_evidence:
// no_deletion_or_contradicting_lifecycle and early_artifact_retrievable.
func VerifyRetentionLate(ctx context.Context, db *sql.DB, api *APIClient, repoRoot string, baseline *RetentionBaseline) (noDelete Verdict, retrievable Verdict, err error) {
	lateCounts, err := tableRowCounts(ctx, db)
	if err != nil {
		return Verdict{}, Verdict{}, err
	}

	var decreased []string
	for _, tbl := range tableOrder {
		if lateCounts[tbl] < baseline.RowCounts[tbl] {
			decreased = append(decreased, fmt.Sprintf("%s: %d -> %d", tbl, baseline.RowCounts[tbl], lateCounts[tbl]))
		}
	}

	var earlySHA []byte
	shaErr := db.QueryRowContext(ctx, `SELECT raw_event_sha256 FROM submissions WHERE id = $1`, baseline.EarlySubmissionID).Scan(&earlySHA)
	earlyStillPresent := shaErr == nil
	earlyUnchanged := earlyStillPresent && string(earlySHA) == string(baseline.EarlySubmissionSHA256)

	staticClean, staticDetail := noDeleteStaticCheck(repoRoot)

	rows, statErr := db.QueryContext(ctx, `SELECT relname, n_tup_del FROM pg_stat_user_tables WHERE relname = ANY($1)`, tableOrder)
	nTupDel := map[string]int64{}
	if statErr == nil {
		for rows.Next() {
			var name string
			var n int64
			if scanErr := rows.Scan(&name, &n); scanErr == nil {
				nTupDel[name] = n
			}
		}
		rows.Close()
	}
	var totalTupDel int64
	for _, n := range nTupDel {
		totalTupDel += n
	}

	status := VerdictPass
	detail := fmt.Sprintf(
		"no table's row count decreased since the early-run baseline (submissions/validation_outcomes/normalized_events/detection_definitions/detection_results/alerts all checked); early submission %d's content (raw_event_sha256) is unchanged. Supplementary: pg_stat_user_tables.n_tup_del totals %d across these tables, and %s.",
		baseline.EarlySubmissionID, totalTupDel, staticDetail)
	if len(decreased) > 0 || !earlyStillPresent || !earlyUnchanged {
		status = VerdictFail
		detail = fmt.Sprintf("retention violation detected: decreased row counts=%v; early submission %d still present=%v, content unchanged=%v (scan err: %v)",
			decreased, baseline.EarlySubmissionID, earlyStillPresent, earlyUnchanged, shaErr)
	}
	noDelete = Verdict{
		Clause: "no_deletion_or_contradicting_lifecycle",
		Status: status,
		Detail: detail,
		Numbers: map[string]any{
			"baselineRowCounts": baseline.RowCounts,
			"lateRowCounts":     lateCounts,
			"nTupDel":           nTupDel,
			"staticCheckClean":  staticClean,
		},
	}

	if baseline.EarlyAlertID == 0 {
		retrievable = Verdict{
			Clause: "early_artifact_retrievable",
			Status: VerdictFail,
			Detail: "no early alert was captured in the baseline (sustained run produced no resolved scenario alert) -- cannot verify late retrievability",
		}
		return noDelete, retrievable, nil
	}

	lateStatus, lateBody, getErr := api.GetJSON(ctx, fmt.Sprintf("/v1/alerts/%d", baseline.EarlyAlertID))
	switch {
	case getErr != nil:
		retrievable = Verdict{
			Clause: "early_artifact_retrievable",
			Status: VerdictFail,
			Detail: fmt.Sprintf("late retrieval of early alert %d failed: %v", baseline.EarlyAlertID, getErr),
		}
	case lateStatus != 200:
		retrievable = Verdict{
			Clause: "early_artifact_retrievable",
			Status: VerdictFail,
			Detail: fmt.Sprintf("late retrieval of early alert %d returned status %d, want 200", baseline.EarlyAlertID, lateStatus),
		}
	case !reflect.DeepEqual(baseline.EarlyAlertResponse, lateBody):
		retrievable = Verdict{
			Clause:  "early_artifact_retrievable",
			Status:  VerdictFail,
			Detail:  fmt.Sprintf("early alert %d's content differs between the early-run snapshot and the late retrieval -- not merely re-existence, actual content changed", baseline.EarlyAlertID),
			Numbers: map[string]any{"early": baseline.EarlyAlertResponse, "late": lateBody},
		}
	default:
		retrievable = Verdict{
			Clause: "early_artifact_retrievable",
			Status: VerdictPass,
			Detail: fmt.Sprintf("early alert %d (captured near the beginning of the sustained run) retrieved successfully near the end via the real GET /v1/alerts/{id} path, with byte-for-byte-equivalent content", baseline.EarlyAlertID),
		}
	}
	return noDelete, retrievable, nil
}
