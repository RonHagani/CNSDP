// Command verification is the CNSDP performance/capacity/recovery
// verification harness (AC-019 through AC-023, AC-028's incidental
// deployment-transcript byproduct). It drives the real, dedicated
// "cnsdp-verify" Docker Compose project -- never the developer's own
// running stack -- through the real HTTP intake and retrieval API, and
// reads the real Postgres instance only through a dedicated read-only
// role, to produce a timestamped JSON report plus a human-readable
// summary. It never modifies product code or product behavior.
//
// Usage:
//
//	go run ./test/verification -profile=smoke
//	go run ./test/verification -profile=full
//
// A smoke run validates harness plumbing only and never emits an
// authoritative AC-019 through AC-023 verdict (see report.go,
// (*Report).SetACResults). Only a full run's report is verification
// evidence.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/google/uuid"
)

func main() {
	os.Exit(run())
}

func run() int {
	profileFlag := flag.String("profile", "smoke", `execution profile: "smoke" or "full"`)
	resultsDirFlag := flag.String("results-dir", "", "directory for the JSON report (default: test/verification/results next to this source)")
	flag.Parse()

	params, err := ParamsFor(Profile(*profileFlag))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return exitEnvironmentError
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runCtx, cancelWatchdog := context.WithTimeout(ctx, params.OverallTimeout)
	defer cancelWatchdog()

	runID := uuid.NewString()[:8]
	report := NewReport(params.Profile, runID)

	resultsDir := *resultsDirFlag
	if resultsDir == "" {
		if root, err := repoRootDir(); err == nil {
			resultsDir = filepath.Join(root, "test", "verification", "results")
		} else {
			resultsDir = "verification-results"
		}
	}

	compose, err := NewCompose(params.CapacityFixtureTmpfsBytes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot start: %v\n", err)
		return exitEnvironmentError
	}
	defer compose.Close()

	// Cleanup is unconditional -- it runs whether this function returns
	// normally, a phase fails, the context is canceled by Ctrl+C/SIGTERM,
	// or the overall watchdog fires, since it is registered via defer
	// before any of those can happen and uses its own background context
	// (never runCtx/ctx, both of which may already be canceled by the
	// time this runs).
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if err := compose.Down(cleanupCtx, true); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup (compose down): %v\n", err)
		}
	}()

	if err := compose.PreflightClean(runCtx); err != nil {
		fmt.Fprintf(os.Stderr, "preflight cleanup (tolerated, nothing may have existed): %v\n", err)
	}

	isolation, err := compose.ValidateIsolation(runCtx)
	if err != nil {
		return abort(report, resultsDir, "validate_isolation", err)
	}
	report.Isolation = isolation
	if !isolation.Satisfied {
		report.Notes = append(report.Notes, "isolation invariant NOT satisfied -- refusing to proceed: "+strings.Join(isolation.Problems, "; "))
		recordPhase(report, "validate_isolation", time.Now(), fmt.Errorf("isolation invariant not satisfied: %s", strings.Join(isolation.Problems, "; ")))
		finalize(report, resultsDir)
		return exitEnvironmentError
	}
	recordPhase(report, "validate_isolation", time.Now(), nil)

	upStart := time.Now()
	upErr := compose.Up(runCtx)
	if !recordPhase(report, "compose_up", upStart, upErr) {
		finalize(report, resultsDir)
		return exitEnvironmentError
	}

	appPort, err := compose.Port(runCtx, "app", 8080)
	if err != nil {
		return abort(report, resultsDir, "discover_app_port", err)
	}
	pgPort, err := compose.Port(runCtx, "postgres", 5432)
	if err != nil {
		return abort(report, resultsDir, "discover_postgres_port", err)
	}

	api := NewAPIClient("http://127.0.0.1:"+appPort, compose.APIToken())
	if !waitReadyBool(runCtx, api, 90*time.Second) {
		return abort(report, resultsDir, "wait_initial_ready", fmt.Errorf("platform did not become ready within 90s of startup"))
	}
	recordPhase(report, "wait_initial_ready", time.Now(), nil)

	db, err := SetupReadOnlyAccess(runCtx, pgPort, compose.PostgresPassword())
	if err != nil {
		return abort(report, resultsDir, "setup_read_only_access", err)
	}
	defer db.Close()
	recordPhase(report, "setup_read_only_access", time.Now(), nil)

	fixtures, err := LoadScenarioFixtures(compose.repoRoot)
	if err != nil {
		return abort(report, resultsDir, "load_fixtures", err)
	}

	containerID, err := compose.ContainerID(runCtx, "app")
	if err != nil {
		return abort(report, resultsDir, "resolve_app_container_id", err)
	}

	stopSampling := StartResourceSampling(runCtx, compose, containerID, params.ResourceSampleInterval)

	log.Printf("phase sustained_run: starting (baseline %s @ %.1f/s, burst %s @ %.1f/s total)", params.BaselineDuration, params.NormalOfferedRate, params.BurstDuration, params.BurstOfferedRate)
	sustainedStart := time.Now()
	sustainedResult, sustainedErr := RunSustainedPhase(runCtx, api, db, fixtures, params, params.PostRunDrainWait)
	resourceSamples := stopSampling()
	if !recordPhase(report, "sustained_run", sustainedStart, sustainedErr) {
		finalize(report, resultsDir)
		return exitEnvironmentError
	}

	log.Printf("phase ac023_capture_retention_baseline: starting")
	baselineStart := time.Now()
	retentionBaseline, baselineErr := CaptureRetentionBaseline(runCtx, db, api, sustainedResult)
	if !recordPhase(report, "ac023_capture_retention_baseline", baselineStart, baselineErr) {
		finalize(report, resultsDir)
		return exitEnvironmentError
	}

	log.Printf("phase verify_no_loss: starting")
	noLossStart := time.Now()
	noLossVerdict, noLossErr := AC022VerifyNoLoss(runCtx, db, sustainedResult)
	if !recordPhase(report, "verify_no_loss", noLossStart, noLossErr) {
		finalize(report, resultsDir)
		return exitEnvironmentError
	}

	var alertIDs []int64
	for _, c := range sustainedResult.Correlations {
		if c.AlertID != nil {
			alertIDs = append(alertIDs, *c.AlertID)
		}
	}

	log.Printf("phase retrieval_latency: starting (%d alert ids available)", len(alertIDs))
	retrievalStart := time.Now()
	retrievalResult, retrievalErr := RunRetrievalLatencyPhase(runCtx, api, params, alertIDs)
	if !recordPhase(report, "retrieval_latency", retrievalStart, retrievalErr) {
		finalize(report, resultsDir)
		return exitEnvironmentError
	}

	log.Printf("phase recovery: starting")
	recoveryStart := time.Now()
	recoveryResult, recoveryErr := RunRecoveryPhase(runCtx, api, compose, db, params)
	if !recordPhase(report, "recovery", recoveryStart, recoveryErr) {
		finalize(report, resultsDir)
		return exitEnvironmentError
	}

	// From here on, AC-019 through AC-022's evidence is fully captured
	// against the untouched primary postgres/app pair. ac023_retention_evidence
	// re-checks retention against that same still-intact primary database,
	// strictly before ac023_storage_exhaustion brings up the destructive,
	// isolated capacity fixture -- so the destructive phase can never
	// contaminate anything above it, even in principle.
	//
	// The app's host port is re-discovered rather than reusing the
	// pre-recovery api client: recovery.go's own documented finding is
	// that Docker does not guarantee the same host port survives a
	// `docker compose start` cycle, and the recovery phase's own
	// port-rediscovery is local to that phase, so this run's api variable
	// is potentially stale for every retrieval call from here on.
	recoveryAppPort, portErr := compose.Port(runCtx, "app", 8080)
	if portErr != nil {
		return abort(report, resultsDir, "rediscover_app_port_post_recovery", portErr)
	}
	api = NewAPIClient("http://127.0.0.1:"+recoveryAppPort, compose.APIToken())

	log.Printf("phase ac023_retention_evidence: starting")
	retentionStart := time.Now()
	noDeleteVerdict, retrievableVerdict, retentionErr := VerifyRetentionLate(runCtx, db, api, compose.repoRoot, retentionBaseline)
	if !recordPhase(report, "ac023_retention_evidence", retentionStart, retentionErr) {
		finalize(report, resultsDir)
		return exitEnvironmentError
	}
	log.Printf("ac023_retention_evidence: no_deletion_or_contradicting_lifecycle=%s (%s) early_artifact_retrievable=%s (%s)",
		noDeleteVerdict.Status, noDeleteVerdict.Detail, retrievableVerdict.Status, retrievableVerdict.Detail)

	log.Printf("phase ac023_storage_exhaustion: starting")
	exhaustionStart := time.Now()
	placementReport, exhaustionOutcome, exhaustionErr := RunStorageExhaustionPhase(runCtx, compose, params)
	if !recordPhase(report, "ac023_storage_exhaustion", exhaustionStart, exhaustionErr) {
		finalize(report, resultsDir)
		return exitEnvironmentError
	}
	// Logged unconditionally (not gated by profile, unlike the AC-023
	// verdict itself): an operator watching a smoke run benefits from
	// seeing whether the mechanism actually triggered, same as every
	// other phase's own progress logging above.
	log.Printf("ac023_storage_exhaustion: placementAllInCapacity=%v placementProblems=%v attemptsSent=%d attemptsAdmitted=%d exhausted=%v observedVia=%q appLogHasMarker=%v postgresLogHasEnospc=%v platformUsableAfter=%v platformUsableDetail=%q",
		placementReport.AllInCapacity, placementReport.Problems, exhaustionOutcome.AttemptsSent, exhaustionOutcome.AttemptsAdmitted,
		exhaustionOutcome.Exhausted, exhaustionOutcome.ObservedVia, exhaustionOutcome.AppLogHasMarker,
		exhaustionOutcome.PostgresLogHasENOSPC, exhaustionOutcome.PlatformUsableAfter, exhaustionOutcome.PlatformUsableDetail)

	if params.Profile == ProfileFull {
		ac020 := ac020Verdict(sustainedResult)
		ac022 := replaceClause(ac022Verdict(params, sustainedResult), noLossVerdict)
		ac021 := ac021Verdict(retrievalResult)
		ac023 := ac023Verdict(resourceSamples, noDeleteVerdict, retrievableVerdict, placementReport, exhaustionOutcome)
		ac019 := ac019Verdict(params, recoveryResult)
		report.SetACResults([]ACResult{ac020, ac021, ac022, ac023, ac019})
	} else {
		report.SetACResults(nil)
	}

	return finalize(report, resultsDir)
}

func replaceClause(ac ACResult, v Verdict) ACResult {
	for i, c := range ac.Clauses {
		if c.Clause == v.Clause {
			ac.Clauses[i] = v
		}
	}
	ac.Overall = rollupOverall(ac.Clauses)
	return ac
}

func recordPhase(report *Report, name string, start time.Time, err error) bool {
	ph := PhaseHealth{Phase: name, StartedAt: start, Duration: time.Since(start).String(), Succeeded: err == nil}
	if err != nil {
		ph.Error = err.Error()
		log.Printf("phase %s: FAILED after %s: %v", name, ph.Duration, err)
	} else {
		log.Printf("phase %s: ok (%s)", name, ph.Duration)
	}
	report.Phases = append(report.Phases, ph)
	return ph.Succeeded
}

// abort records a failed phase and writes/prints whatever report state
// exists so far before returning the environment-error exit code -- a
// harness-level failure always yields a report, never a silent nonzero
// exit with no artifact to inspect.
func abort(report *Report, resultsDir, phase string, err error) int {
	recordPhase(report, phase, time.Now(), err)
	finalize(report, resultsDir)
	return exitEnvironmentError
}

func finalize(report *Report, resultsDir string) int {
	path, err := report.WriteJSON(resultsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "write report: %v\n", err)
	} else {
		fmt.Println(report.HumanSummary())
		fmt.Println("report written to:", path)
	}
	if err != nil {
		return exitEnvironmentError
	}
	return report.ExitCode()
}

func waitReadyBool(ctx context.Context, api *APIClient, budget time.Duration) bool {
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		if api.Ready(ctx) {
			return true
		}
		sleepUntil(ctx, time.Now().Add(500*time.Millisecond))
		if ctx.Err() != nil {
			return false
		}
	}
	return false
}
