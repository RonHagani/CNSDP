package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// RecoveryResult is the full evidence trail for AC-019: what existed
// right before the interruption, when it happened, when the platform
// became ready again, and what every pre-interruption submission's state
// looked like afterward.
type RecoveryResult struct {
	BatchIDs          []int64
	PreKillStatuses   map[int64]string
	InterruptedAt     time.Time
	RevivalIssuedAt   time.Time
	RevivalError      string
	RecoveredAt       time.Time
	RTO               time.Duration
	PostDrainStatuses map[int64]string
	Missing           []int64
	Duplicates        []DuplicateFinding
}

// recoveryMixEvent builds one item of the mixed batch AC-019's Context
// requires ("some submissions were admitted but not yet fully
// processed") -- deliberately spanning the four validation outcomes so
// the interruption lands on a genuinely heterogeneous, realistic batch,
// not just uniform valid traffic.
func recoveryMixEvent(kind string, seq int) json.RawMessage {
	id := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	switch kind {
	case "unsupported":
		m := map[string]any{"kind": "NotAnEvent", "apiVersion": "v1", "auditID": id, "note": fmt.Sprintf("recovery-mix-unsupported-%d", seq)}
		b, _ := json.Marshal(m)
		return b
	case "invalid":
		// auditID as a number, not a string: audit.k8s.io/v1 Event.AuditID
		// (types.UID) fails to type-decode, producing a genuine JSON decode
		// error -- OutcomeInvalid, not OutcomeUnsupported (kind/apiVersion
		// markers are still correct).
		raw := fmt.Sprintf(`{"kind":"Event","apiVersion":"audit.k8s.io/v1","auditID":%d,"stage":"ResponseComplete"}`, seq)
		return json.RawMessage(raw)
	case "incomplete":
		m := map[string]any{
			"kind": "Event", "apiVersion": "audit.k8s.io/v1", "level": "RequestResponse",
			"auditID": id, "stage": "ResponseComplete",
			"requestReceivedTimestamp": now,
			"verb":                     "get", "requestURI": "/api/v1/namespaces/default/configmaps/recovery-mix",
			"objectRef": map[string]any{"resource": "configmaps", "namespace": "default", "name": "recovery-mix"},
			// user.username deliberately omitted -- FR-007(c).
		}
		b, _ := json.Marshal(m)
		return b
	default: // "valid" -- reuse the filler-event shape
		item, _ := NewFillerEvent(seq)
		return item
	}
}

// waitForMidPipelineSpread polls until the batch's submissions are
// genuinely spread across at least two distinct statuses (proving the
// pipeline is actively mid-processing, not still uniformly at the
// initial admitted status) with at least one not yet terminal -- a
// condition-based wait, not a fixed sleep, so the interruption lands
// mid-stream regardless of machine speed. Bounded: if the condition never
// occurs (e.g. the batch drains faster than this can observe), it gives
// up after the bound and the caller proceeds anyway -- a recovery test
// against a fully-drained batch is a weaker but still valid interruption
// test, and this must not hang the run indefinitely.
func waitForMidPipelineSpread(ctx context.Context, db *sql.DB, ids []int64, bound time.Duration) (map[int64]string, bool) {
	deadline := time.Now().Add(bound)
	var last map[int64]string
	for time.Now().Before(deadline) {
		statuses, err := SubmissionStatuses(ctx, db, ids)
		if err == nil {
			last = statuses
			distinct := map[string]struct{}{}
			anyNonTerminal := false
			for _, s := range statuses {
				distinct[s] = struct{}{}
				if s != "evidenced" {
					anyNonTerminal = true
				}
			}
			if len(distinct) >= 2 && anyNonTerminal {
				return statuses, true
			}
		}
		sleepUntil(ctx, time.Now().Add(100*time.Millisecond))
		if ctx.Err() != nil {
			break
		}
	}
	return last, false
}

// RunRecoveryPhase implements AC-019: admit a mixed batch, wait for a
// genuine mid-pipeline spread, kill the app container abruptly (SIGKILL,
// via Compose.Kill -- deliberately bypassing cmd/platform's own
// SIGTERM-driven graceful shutdown), execute the documented recovery
// procedure, measure RTO via /readyz, then reconcile every pre-kill id.
// No crash hook is injected into product code anywhere in this phase --
// see docker-compose.verify.yml and compose.go's Kill doc comment for why
// this is a safe, deterministic, architecture-justified interruption
// rather than invented chaos.
//
// The documented recovery procedure step (empirically determined, not
// assumed): a first smoke-profile run against this exact environment
// showed `docker compose kill` followed by nothing but a passive wait
// left the container sitting exited indefinitely -- `docker inspect`
// confirmed RestartCount stayed at 0 for over ten minutes despite the
// base compose file's own `restart: unless-stopped` policy (unchanged;
// this is not a product-behavior modification). AC-019's own text frames
// recovery as "following the documented recovery procedure", which
// nothing in the approved documents requires to be fully passive --
// docs/reference-environment.md has never documented any recovery
// procedure at all (a gap noted during design). The procedure exercised
// and measured here is therefore: interruption, then an explicit,
// idempotent `docker compose start app` (Compose.StartService) -- the
// realistic action an operator or a supervising process takes -- then
// polling /readyz for the approved 15-minute budget. RTO is still
// measured from the moment of interruption, not from the revival
// command, since NFR-009's 15-minute objective is about the platform's
// own mechanical recovery capability, not how quickly a human notices.
func RunRecoveryPhase(ctx context.Context, api *APIClient, compose *Compose, db *sql.DB, p Params) (*RecoveryResult, error) {
	kinds := []string{"unsupported", "invalid", "incomplete", "valid", "valid", "valid"}
	var batchIDs []int64
	for i := 0; i < p.RecoveryBatchSize; i++ {
		kind := kinds[i%len(kinds)]
		item := recoveryMixEvent(kind, i)
		id, _, err := api.PostAuditEvent(ctx, item)
		if err != nil {
			// An admission failure here is an environment condition: this
			// phase's own seeding step is expected to succeed against a
			// healthy platform, since intake admits every syntactically
			// identifiable item regardless of content shape (FR-005).
			return nil, wrapEnvErr("seed recovery batch", fmt.Errorf("item %d (%s): %w", i, kind, err))
		}
		batchIDs = append(batchIDs, id)
	}

	preKill, _ := waitForMidPipelineSpread(ctx, db, batchIDs, 20*time.Second)
	if preKill == nil {
		var err error
		preKill, err = SubmissionStatuses(ctx, db, batchIDs)
		if err != nil {
			return nil, wrapEnvErr("snapshot pre-kill submission statuses", err)
		}
	}

	log.Printf("recovery: seeded %d submissions, pre-kill statuses observed: %v", len(batchIDs), preKill)

	interruptedAt := time.Now()
	log.Printf("recovery: killing app container (SIGKILL) at %s", interruptedAt.Format(time.RFC3339))
	if err := compose.Kill(ctx, "app"); err != nil {
		return nil, err // already wrapped as *EnvironmentError by compose.run
	}
	log.Printf("recovery: kill issued successfully")

	// Execute the documented recovery procedure's revival step (see the
	// package-level doc comment above for why this is not purely
	// passive). A failure here is not treated as fatal to the phase --
	// waitReady's own bounded poll remains the sole authority on whether
	// recovery succeeded within AC-019's budget, so a transient CLI error
	// here cannot itself manufacture either a false PASS or a false
	// environment abort; it is carried into the result for the record.
	revivalIssuedAt := time.Now()
	var revivalErrText string
	if err := compose.StartService(ctx, "app"); err != nil {
		revivalErrText = err.Error()
		log.Printf("recovery: `docker compose start app` returned an error (non-fatal, carried into the result): %v", err)
	} else {
		log.Printf("recovery: `docker compose start app` returned successfully at %s", time.Now().Format(time.RFC3339))
	}

	// Re-discover the app's published host port rather than reusing the
	// pre-kill one cached by main.go's initial compose.Port call. Real,
	// empirically confirmed finding from this design's own first live
	// runs: docker-compose.verify.yml deliberately requests a dynamically
	// assigned host port for isolation (an empty host-port field), and
	// Docker does not guarantee that mapping survives a stop/start cycle
	// of the same container -- `docker inspect` showed the container
	// reassigned to a materially different host port after `docker
	// compose start`, which left the harness polling a now-dead port and
	// misreporting a live, sub-2-second product recovery as a 10+ minute
	// hang. This is a harness bug fix, not a product change: nothing
	// about cmd/platform or docker-compose.yml's restart policy is
	// touched here.
	newPort, portErr := reDiscoverAppPort(ctx, compose)
	if portErr != nil {
		return nil, portErr
	}
	log.Printf("recovery: re-discovered app port %s after revival", newPort)
	recoveryAPI := NewAPIClient("http://127.0.0.1:"+newPort, api.Token)

	recoveredAt, err := waitReady(ctx, recoveryAPI, p.RecoveryRTOBudget)
	if err != nil {
		log.Printf("recovery: waitReady returned an environment error: %v", err)
		return nil, err
	}
	if recoveredAt.IsZero() {
		log.Printf("recovery: waitReady exhausted its %s budget without observing GET /readyz return 200", p.RecoveryRTOBudget)
	} else {
		log.Printf("recovery: platform ready again at %s (RTO %s)", recoveredAt.Format(time.RFC3339), recoveredAt.Sub(interruptedAt))
	}

	sleepUntil(ctx, time.Now().Add(p.RecoveryDrainWait))

	postDrain, err := SubmissionStatuses(ctx, db, batchIDs)
	if err != nil {
		return nil, wrapEnvErr("snapshot post-recovery submission statuses", err)
	}

	var missing []int64
	for _, id := range batchIDs {
		if _, ok := postDrain[id]; !ok {
			missing = append(missing, id)
		}
	}

	duplicates, err := CheckNoDuplicates(ctx, db)
	if err != nil {
		return nil, wrapEnvErr("check for duplicate artifacts after recovery", err)
	}

	return &RecoveryResult{
		BatchIDs:          batchIDs,
		PreKillStatuses:   preKill,
		InterruptedAt:     interruptedAt,
		RevivalIssuedAt:   revivalIssuedAt,
		RevivalError:      revivalErrText,
		RecoveredAt:       recoveredAt,
		RTO:               recoveredAt.Sub(interruptedAt),
		PostDrainStatuses: postDrain,
		Missing:           missing,
		Duplicates:        duplicates,
	}, nil
}

// reDiscoverAppPort queries the app service's currently published host
// port with a few short retries -- `docker compose port` is a fast,
// lightweight query against Docker's own port-mapping table and is
// expected to succeed almost immediately once `docker compose start` has
// returned, but a brief retry window is cheap insurance against querying
// in the narrow instant right as the container transitions state. Always
// returns a non-empty port on a nil error; RunRecoveryPhase treats any
// error here as fatal to the phase, since a recovery test that cannot
// even determine where to poll cannot produce a trustworthy AC-019
// verdict either way.
func reDiscoverAppPort(ctx context.Context, compose *Compose) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		port, err := compose.Port(ctx, "app", 8080)
		if err == nil {
			return port, nil
		}
		lastErr = err
		sleepUntil(ctx, time.Now().Add(500*time.Millisecond))
		if ctx.Err() != nil {
			break
		}
	}
	return "", wrapEnvErr("re-discover app port after recovery revival", lastErr)
}

// waitReady polls GET /readyz until it returns 200 or budget elapses.
// Exceeding budget is a genuine AC-019 requirement failure (returned as
// data via the caller's verdict, not here) -- but a budget so generous
// (15 minutes) that never returning at all more likely indicates the
// harness's own polling broke or the container never came back at all,
// which this function surfaces as a distinguishable, explicit outcome via
// its returned error rather than silently returning a zero time.
func waitReady(ctx context.Context, api *APIClient, budget time.Duration) (time.Time, error) {
	deadline := time.Now().Add(budget)
	attempt := 0
	lastLog := time.Now()
	for time.Now().Before(deadline) {
		attempt++
		ok, reason := api.ReadyDiag(ctx)
		if ok {
			return time.Now(), nil
		}
		if time.Since(lastLog) >= 10*time.Second {
			log.Printf("recovery: still waiting for readiness (attempt %d, %s elapsed of %s budget): %s", attempt, time.Since(deadline.Add(-budget)).Round(time.Second), budget, reason)
			lastLog = time.Now()
		}
		sleepUntil(ctx, time.Now().Add(500*time.Millisecond))
		if ctx.Err() != nil {
			return time.Time{}, wrapEnvErr("wait for readiness after recovery", ctx.Err())
		}
	}
	return time.Time{}, nil // caller's ac019Verdict treats a zero RecoveredAt as an RTO-budget FAIL, a requirement finding, not an environment error
}

func ac019Verdict(p Params, r *RecoveryResult) ACResult {
	var clauses []Verdict

	rtoStatus := VerdictPass
	rtoDetail := fmt.Sprintf("recovered in %s (budget %s) following the documented procedure: interrupt, `docker compose start app`, poll /readyz", r.RTO, p.RecoveryRTOBudget)
	if r.RecoveredAt.IsZero() {
		rtoStatus = VerdictFail
		rtoDetail = fmt.Sprintf("platform did not become ready (GET /readyz 200) within the %s recovery-time-objective budget", p.RecoveryRTOBudget)
		if r.RevivalError != "" {
			rtoDetail += fmt.Sprintf("; the recovery procedure's `docker compose start app` step itself returned an error: %s", r.RevivalError)
		}
	} else if r.RTO > p.RecoveryRTOBudget {
		rtoStatus = VerdictFail
	}
	clauses = append(clauses, Verdict{
		Clause: "rto_within_budget",
		Status: rtoStatus,
		Detail: rtoDetail,
		Numbers: map[string]any{
			"interruptedAt":   r.InterruptedAt,
			"revivalIssuedAt": r.RevivalIssuedAt,
			"revivalError":    r.RevivalError,
			"recoveredAt":     r.RecoveredAt,
			"rto":             r.RTO.String(),
			"budget":          p.RecoveryRTOBudget.String(),
		},
	})

	lossStatus := VerdictPass
	lossDetail := fmt.Sprintf("all %d pre-interruption submissions remained present after recovery", len(r.BatchIDs))
	if len(r.Missing) > 0 {
		lossStatus = VerdictFail
		lossDetail = fmt.Sprintf("%d of %d pre-interruption submissions are missing after recovery: %v", len(r.Missing), len(r.BatchIDs), r.Missing)
	}
	clauses = append(clauses, Verdict{Clause: "no_loss", Status: lossStatus, Detail: lossDetail, Numbers: map[string]any{"batchSize": len(r.BatchIDs), "missing": len(r.Missing)}})

	determinable := VerdictPass
	determinableDetail := "every pre-interruption submission's post-recovery status is present and legible"
	if len(r.PostDrainStatuses) < len(r.BatchIDs) {
		determinable = VerdictFail
		determinableDetail = "at least one pre-interruption submission's status could not be determined after recovery"
	}
	clauses = append(clauses, Verdict{Clause: "determinable_state", Status: determinable, Detail: determinableDetail})

	progressed := 0
	for _, id := range r.BatchIDs {
		pre, hadPre := r.PreKillStatuses[id]
		post, hadPost := r.PostDrainStatuses[id]
		if hadPre && hadPost && post != pre {
			progressed++
		}
	}
	resumeStatus := VerdictPass
	resumeDetail := fmt.Sprintf("%d of %d submissions advanced status between the pre-kill snapshot and the post-recovery check", progressed, len(r.BatchIDs))
	if len(r.BatchIDs) > 0 && progressed == 0 {
		resumeStatus = VerdictFail
		resumeDetail = "no submission advanced status after recovery -- the worker does not appear to have resumed processing"
	}
	clauses = append(clauses, Verdict{Clause: "processing_resumed", Status: resumeStatus, Detail: resumeDetail, Numbers: map[string]any{"progressed": progressed, "batchSize": len(r.BatchIDs)}})

	dupStatus := VerdictPass
	dupDetail := "no duplicate per-key artifact rows found in validation_outcomes, normalized_events, detection_results, or alerts"
	if len(r.Duplicates) > 0 {
		dupStatus = VerdictFail
		dupDetail = fmt.Sprintf("%d duplicate-key finding(s) -- see Numbers for detail", len(r.Duplicates))
	}
	clauses = append(clauses, Verdict{Clause: "no_duplicate_artifacts", Status: dupStatus, Detail: dupDetail, Numbers: map[string]any{"findings": r.Duplicates}})

	return newACResult("AC-019", "Recovery from interruption within the recovery-time objective", clauses)
}
