package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"
)

// plannedAttempt is one pre-built workload item: which scenario it should
// accumulate toward (empty for filler), and its already-cloned event
// bytes (a fresh, unique auditID per item -- see fixtures.go). Built once,
// entirely before dispatch begins, so the concurrent send loop never
// makes a scenario/filler decision itself -- eliminates a whole class of
// shared-mutable-state races between planning and sending.
type plannedAttempt struct {
	Scenario ScenarioID
	Item     json.RawMessage
}

// buildWorkloadPlan lays out exactly p.TotalOffered() items: scenario
// slots evenly spaced (round-robin across the three scenarios) so
// matching submissions are sent throughout the run rather than
// front-loaded, and filler everywhere else. A fixup pass guarantees
// exactly p.MinMatchingPerScenario per scenario even after integer
// rounding of the spacing.
func buildWorkloadPlan(fixtures *ScenarioFixtures, p Params) ([]plannedAttempt, error) {
	total := p.TotalOffered()
	plan := make([]plannedAttempt, total)

	order := []ScenarioID{Scenario1, Scenario2, Scenario3}
	target := p.MinMatchingPerScenario
	totalSlots := target * len(order)
	slotEvery := total / totalSlots
	if slotEvery < 1 {
		slotEvery = 1
	}

	sent := map[ScenarioID]int{}
	rr := 0
	fillerSeq := 0
	for n := 0; n < total; n++ {
		assign := ScenarioID("")
		if n%slotEvery == 0 {
			for i := 0; i < len(order); i++ {
				cand := order[rr%len(order)]
				rr++
				if sent[cand] < target {
					assign = cand
					sent[cand]++
					break
				}
			}
		}
		if assign != "" {
			item, _, err := fixtures.Clone(assign)
			if err != nil {
				return nil, fmt.Errorf("clone %s: %w", assign, err)
			}
			plan[n] = plannedAttempt{Scenario: assign, Item: item}
		} else {
			item, _ := NewFillerEvent(fillerSeq)
			fillerSeq++
			plan[n] = plannedAttempt{Item: item}
		}
	}

	// Fixup: convert trailing filler slots to any scenario short of its
	// target (only possible if slotEvery rounded unfavorably for a small
	// total, e.g. the smoke profile).
	for _, sid := range order {
		for sent[sid] < target {
			converted := false
			for n := total - 1; n >= 0; n-- {
				if plan[n].Scenario == "" {
					item, _, err := fixtures.Clone(sid)
					if err != nil {
						return nil, fmt.Errorf("clone %s (fixup): %w", sid, err)
					}
					plan[n] = plannedAttempt{Scenario: sid, Item: item}
					sent[sid]++
					converted = true
					break
				}
			}
			if !converted {
				return nil, fmt.Errorf("workload plan too small to fit %d attempts per scenario (total=%d)", target, total)
			}
		}
	}
	return plan, nil
}

// Attempt is the harness's own record of one dispatched HTTP POST --
// authoritative for "offered" (every Attempt) and "admitted" (OK==true)
// counts. Two distinct clocks are recorded, deliberately kept separate:
//
//   - SentAt is the workload plan's *scheduled* target time (an even,
//     drift-free sequence derived from a fixed start and the offered
//     rate) -- used to determine which experimental condition (baseline
//     vs. burst rate) an attempt belongs to, e.g. BurstStartIdx below.
//     This is a test-design concept, unaffected by how dispatch actually
//     went, and must stay that way: partitioning baseline vs. burst by
//     real time instead would misclassify jitter-delayed baseline
//     attempts as burst and vice versa.
//   - ActualSentAt is when this attempt's HTTP exchange actually began
//     in real wall-clock time, captured immediately before the
//     PostAuditEvent call (after any dispatch/semaphore wait). Real
//     dispatch can lag or cluster relative to the schedule (goroutine
//     scheduling, semaphore backpressure, network/OS timer jitter), so
//     this can differ from SentAt. Any check that reasons about the
//     product's own real-time admission behavior -- notably
//     ac022AdmittedRateEnvelope's bucketing -- must use this field, not
//     SentAt, since the product's limiter itself decides admission
//     against real time.
type Attempt struct {
	N            int
	SentAt       time.Time
	ActualSentAt time.Time
	Scenario     ScenarioID
	SubmissionID int64
	OK           bool
	StatusCode   int
	Err          string
}

// StatusSample is one point of the backlog/queue-depth time series
// (design item 4, "backlog behavior").
type StatusSample struct {
	At     time.Time
	Counts map[string]int64
}

// SustainedRunResult is the raw evidence the sustained-run phase
// produces; ac020Verdict/ac022Verdict (below) turn it into reported
// clauses -- kept separate so the phase's mechanics and its
// interpretation against approved thresholds are independently reviewable
// and testable.
type SustainedRunResult struct {
	Plan          []plannedAttempt
	Attempts      []Attempt
	StatusSeries  []StatusSample
	BurstStartIdx int // index into Attempts' SentAt-sorted order where the burst window begins
	Correlations  map[int64]AlertCorrelation
}

const dispatchConcurrency = 64

// RunSustainedPhase executes the one combined AC-020/AC-022 experiment:
// AC-022's exact shape (BaselineDuration @ NormalOfferedRate, then
// BurstDuration @ BurstOfferedRate total, offered as one HTTP POST per
// attempt), sampling backlog depth throughout, then -- after a bounded
// drain wait -- resolving every scenario-tagged attempt's alert
// correlation for AC-020's latency measurement. It never returns a Go
// error for a measured threshold violation; only a genuine environment
// problem (compose exec, DB, or repeated HTTP transport failure) does.
func RunSustainedPhase(ctx context.Context, api *APIClient, db *sql.DB, fixtures *ScenarioFixtures, p Params, drainWait time.Duration) (*SustainedRunResult, error) {
	plan, err := buildWorkloadPlan(fixtures, p)
	if err != nil {
		return nil, wrapEnvErr("build sustained-run workload plan", err)
	}

	resultsCh := make(chan Attempt, len(plan))
	sem := make(chan struct{}, dispatchConcurrency)
	inFlight := make(chan struct{}, len(plan))

	sampleCtx, cancelSampling := context.WithCancel(ctx)
	seriesCh := make(chan StatusSample, 8192)
	samplingDone := make(chan struct{})
	go func() {
		defer close(samplingDone)
		ticker := time.NewTicker(p.RateBucketWidth)
		defer ticker.Stop()
		for {
			select {
			case <-sampleCtx.Done():
				return
			case <-ticker.C:
				counts, err := StatusCounts(sampleCtx, db)
				if err == nil {
					seriesCh <- StatusSample{At: time.Now(), Counts: counts}
				}
			}
		}
	}()

	dispatchOne := func(idx int, sentAt time.Time) {
		inFlight <- struct{}{}
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			defer func() { <-inFlight }()
			item := plan[idx]
			actualSentAt := time.Now()
			subID, status, _, postErr := api.PostAuditEvent(ctx, item.Item)
			a := Attempt{N: idx, SentAt: sentAt, ActualSentAt: actualSentAt, Scenario: item.Scenario, SubmissionID: subID, OK: postErr == nil, StatusCode: status}
			if postErr != nil {
				a.Err = postErr.Error()
			}
			resultsCh <- a
		}()
	}

	dispatchWindow := func(rate float64, duration time.Duration, offset int) (nextOffset int, windowStart time.Time) {
		if rate <= 0 || duration <= 0 {
			return offset, time.Now()
		}
		interval := time.Duration(float64(time.Second) / rate)
		start := time.Now()
		n := 0
		for {
			elapsed := time.Duration(n) * interval
			if elapsed >= duration {
				break
			}
			idx := offset + n
			if idx >= len(plan) {
				break
			}
			target := start.Add(elapsed)
			sleepUntil(ctx, target)
			if ctx.Err() != nil {
				break
			}
			dispatchOne(idx, target)
			n++
		}
		return offset + n, start
	}

	baselineOffset, _ := dispatchWindow(p.NormalOfferedRate, p.BaselineDuration, 0)
	_, burstStart := dispatchWindow(p.BurstOfferedRate, p.BurstDuration, baselineOffset)

	// Wait for every dispatched goroutine to finish and release its
	// inFlight token before closing the results channel.
	drainInFlight(inFlight)

	cancelSampling()
	<-samplingDone
	close(seriesCh)
	close(resultsCh)

	var attempts []Attempt
	for a := range resultsCh {
		attempts = append(attempts, a)
	}
	sort.Slice(attempts, func(i, j int) bool { return attempts[i].SentAt.Before(attempts[j].SentAt) })

	var series []StatusSample
	for s := range seriesCh {
		series = append(series, s)
	}
	sort.Slice(series, func(i, j int) bool { return series[i].At.Before(series[j].At) })

	// Bounded post-run drain wait, sampling status once more at the end so
	// the backlog series shows the drain tail, not just the load window.
	sleepUntil(ctx, time.Now().Add(drainWait))
	if counts, err := StatusCounts(ctx, db); err == nil {
		series = append(series, StatusSample{At: time.Now(), Counts: counts})
	}

	var scenarioIDs []int64
	for _, a := range attempts {
		if a.OK && a.Scenario != "" {
			scenarioIDs = append(scenarioIDs, a.SubmissionID)
		}
	}
	correlations, err := CorrelateAlerts(ctx, db, scenarioIDs)
	if err != nil {
		return nil, wrapEnvErr("correlate sustained-run alerts", err)
	}

	burstStartIdx := len(attempts) // default: no attempt fell within the burst window
	for i, a := range attempts {
		if !a.SentAt.Before(burstStart) {
			burstStartIdx = i
			break
		}
	}

	return &SustainedRunResult{
		Plan:          plan,
		Attempts:      attempts,
		StatusSeries:  series,
		BurstStartIdx: burstStartIdx,
		Correlations:  correlations,
	}, nil
}

// drainInFlight blocks until every dispatched goroutine has released its
// inFlight token (len(inFlight) back to zero) -- a buffered channel used
// as a counting semaphore, polled rather than range-received since
// producers (dispatchOne) and this consumer run concurrently with no
// fixed total known in advance to range over.
func drainInFlight(inFlight chan struct{}) {
	for len(inFlight) != 0 {
		time.Sleep(20 * time.Millisecond)
	}
}

// ---- AC-020: intake-to-alert latency -------------------------------------

func ac020Verdict(result *SustainedRunResult) ACResult {
	type sample struct {
		scenario ScenarioID
		latency  time.Duration
	}
	var resolved []sample
	unresolved := map[ScenarioID]int{}

	for _, a := range result.Attempts {
		if !a.OK || a.Scenario == "" {
			continue
		}
		c, ok := result.Correlations[a.SubmissionID]
		if !ok || c.AlertID == nil {
			unresolved[a.Scenario]++
			continue
		}
		resolved = append(resolved, sample{scenario: a.Scenario, latency: c.AlertAt.Sub(c.AdmittedAt)})
	}

	perScenarioCount := map[ScenarioID]int{}
	durations := make([]time.Duration, 0, len(resolved))
	var maxByScenario = map[ScenarioID]time.Duration{}
	for _, s := range resolved {
		durations = append(durations, s.latency)
		perScenarioCount[s.scenario]++
		if s.latency > maxByScenario[s.scenario] {
			maxByScenario[s.scenario] = s.latency
		}
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

	totalUnresolved := 0
	for _, n := range unresolved {
		totalUnresolved += n
	}

	numbers := map[string]any{
		"resolvedCount":        len(durations),
		"unresolvedCount":      totalUnresolved,
		"perScenarioCount":     countMapToStringKeys(perScenarioCount),
		"unresolvedByScenario": countMapToStringKeys(unresolved),
	}
	if len(durations) > 0 {
		numbers["min"] = durations[0].String()
		numbers["p50"] = percentile(durations, 50).String()
		numbers["p95"] = percentile(durations, 95).String()
		numbers["p99"] = percentile(durations, 99).String()
		numbers["max"] = durations[len(durations)-1].String()
	}

	status := VerdictPass
	detail := fmt.Sprintf("%d matching submissions resolved to an alert; max latency %s", len(durations), maxOf(durations))
	if len(durations) == 0 {
		status = VerdictFail
		detail = "no matching submission resolved to an alert within the drain wait"
	} else if durations[len(durations)-1] > 60*time.Second {
		status = VerdictFail
		detail = fmt.Sprintf("all-samples rule violated: max latency %s exceeds the 60s target", durations[len(durations)-1])
	}
	if totalUnresolved > 0 {
		status = VerdictFail
		detail += fmt.Sprintf("; %d scenario submission(s) never resolved to an alert within the drain wait (treated as exceeding the 60s target)", totalUnresolved)
	}
	for _, sid := range []ScenarioID{Scenario1, Scenario2, Scenario3} {
		if perScenarioCount[sid]+unresolved[sid] < 20 {
			status = VerdictFail
			detail += fmt.Sprintf("; %s produced only %d matching submissions (want >=20 per AC-020)", sid, perScenarioCount[sid]+unresolved[sid])
		}
	}

	clause := Verdict{
		Clause:  "all_samples_within_60s",
		Status:  status,
		Detail:  detail,
		Numbers: numbers,
	}
	return newACResult("AC-020", "Intake-to-alert latency of 60 seconds or less", []Verdict{clause})
}

func maxOf(d []time.Duration) time.Duration {
	if len(d) == 0 {
		return 0
	}
	return d[len(d)-1]
}

func countMapToStringKeys[K ~string](m map[K]int) map[string]int {
	out := make(map[string]int, len(m))
	for k, v := range m {
		out[string(k)] = v
	}
	return out
}

// ---- AC-022: sustained capacity envelope ----------------------------------

// rateTolerance is a harness-level documented decision, not a value
// AC-022 itself specifies: the approved text gives no numeric tolerance
// band for "remains within the supported envelope", so a bucket is
// flagged only when it deviates by more than this fraction from the
// target rate.
const rateTolerance = 0.15

func ac022Verdict(p Params, result *SustainedRunResult) ACResult {
	clauses := []Verdict{
		ac022AdmittedRateEnvelope(p, result),
		ac022OfferedAdmittedCounts(p, result),
		ac022CapacityRejectionDefer(p, result),
		ac022NoSilentLoss(result),
		ac022BacklogDrain(result),
		{
			Clause: "dependent_nfr_035",
			Status: VerdictInformational,
			Detail: "NFR-035 (bounded resource consumption) is reported separately under AC-023 as EVIDENCE_ONLY -- no threshold is defined to adjudicate it here or there",
		},
	}
	return newACResult("AC-022", "Sustained capacity envelope of 10 submissions per second", clauses)
}

// ac022AdmittedRateEnvelope buckets by ActualSentAt (real dispatch time),
// deliberately not SentAt (the scheduled target): this clause exists to
// check whether the product's own real-time admission decisions ever
// exceeded the envelope, and the product's limiter itself decides
// admission against real wall-clock time, not against this harness's
// dispatch schedule. Bucketing by SentAt instead would be close to
// tautological for a steady-rate schedule -- a fixed-rate, drift-free
// schedule can never itself show more than its own configured rate when
// grouped by its own clock, regardless of how bursty real dispatch
// actually was, which would silently defeat the purpose of this check.
func ac022AdmittedRateEnvelope(p Params, result *SustainedRunResult) Verdict {
	if len(result.Attempts) == 0 {
		return Verdict{Clause: "admitted_rate_envelope", Status: VerdictFail, Detail: "no attempts recorded"}
	}
	start := result.Attempts[0].ActualSentAt
	bucketWidth := p.RateBucketWidth
	buckets := map[int]int{}
	var maxDispatchLag time.Duration
	for _, a := range result.Attempts {
		if lag := a.ActualSentAt.Sub(a.SentAt); lag > maxDispatchLag {
			maxDispatchLag = lag
		}
		if !a.OK {
			continue
		}
		idx := int(a.ActualSentAt.Sub(start) / bucketWidth)
		buckets[idx]++
	}

	var violations []string
	worstRate := 0.0
	for idx, count := range buckets {
		rate := float64(count) / bucketWidth.Seconds()
		if rate > worstRate {
			worstRate = rate
		}
		if rate > p.NormalOfferedRate*(1+rateTolerance) {
			bucketStart := start.Add(time.Duration(idx) * bucketWidth)
			violations = append(violations, fmt.Sprintf("bucket@%s: %.1f/s", bucketStart.Format("15:04:05.000"), rate))
		}
	}

	status := VerdictPass
	detail := fmt.Sprintf("admitted rate stayed within %.0f%% of the %.0f/s envelope in every %s bucket", rateTolerance*100, p.NormalOfferedRate, bucketWidth)
	if len(violations) > 0 {
		status = VerdictFail
		detail = fmt.Sprintf("admitted rate exceeded the %.0f/s envelope by more than %.0f%% in %d bucket(s) -- see capacity_rejection_defer for the corresponding rejection/defer evidence: %s",
			p.NormalOfferedRate, rateTolerance*100, len(violations), joinLimited(violations, 6))
	}
	return Verdict{
		Clause: "admitted_rate_envelope",
		Status: status,
		Detail: detail,
		Numbers: map[string]any{
			"toleranceFraction": rateTolerance,
			"bucketWidth":       bucketWidth.String(),
			"worstBucketRate":   worstRate,
			"violatingBuckets":  len(violations),
			"maxDispatchLag":    maxDispatchLag.String(),
		},
	}
}

func ac022OfferedAdmittedCounts(p Params, result *SustainedRunResult) Verdict {
	offered, admitted := 0, 0
	burstOffered, burstAdmitted := 0, 0
	for i, a := range result.Attempts {
		offered++
		if a.OK {
			admitted++
		}
		if i >= result.BurstStartIdx {
			burstOffered++
			if a.OK {
				burstAdmitted++
			}
		}
	}
	return Verdict{
		Clause: "offered_admitted_counts",
		Status: VerdictInformational,
		Detail: fmt.Sprintf("offered=%d admitted=%d (burst window: offered=%d admitted=%d)", offered, admitted, burstOffered, burstAdmitted),
		Numbers: map[string]any{
			"expectedOfferedTotal":  p.TotalOffered(),
			"expectedOfferedNormal": p.TotalOfferedNormal(),
			"expectedOfferedBurst":  p.TotalOfferedBurst(),
			"expectedExcessBurst":   p.ExcessBurstAttempts(),
			"actualOfferedTotal":    offered,
			"actualAdmittedTotal":   admitted,
			"actualBurstOffered":    burstOffered,
			"actualBurstAdmitted":   burstAdmitted,
		},
	}
}

// ac022CapacityRejectionDefer is a real PASS/FAIL check: internal/intake
// now defines a capacity-specific status code (429 Too Many Requests,
// admission.go's process-global sliding-window limiter), so this clause no
// longer needs to hedge with an undifferentiated non-OK count the way it
// did before that mechanism existed.
//
// rejectionTolerance mirrors rateTolerance immediately above: AC-022's
// approved text states no numeric tolerance for how closely the observed
// 429 count must reconcile with the expected excess, so this is the same
// kind of harness-level measurement decision, documented the same way --
// concurrent dispatch (dispatchConcurrency) means requests do not land at
// the server in perfectly scheduled order, so a small amount of slack
// against the theoretical expectedExcess is a measurement accommodation,
// not a weakening of AC-022's own pass condition ("remains within the
// supported envelope throughout"), which the separate
// admitted_rate_envelope clause enforces independently and exactly.
//
// The band is symmetric -- a lowerBound and an upperBound, both derived
// from the same tolerance -- so this clause catches both directions of a
// broken admission-control mechanism: too few 429s (under-enforcing,
// admitting more than the envelope allows) and too many 429s
// (over-enforcing, refusing burst-window attempts that were still within
// the approved 10/s envelope and should have been admitted). A lower-bound
// -only check would silently PASS an over-restrictive mechanism.
const rejectionTolerance = 0.15

func ac022CapacityRejectionDefer(p Params, result *SustainedRunResult) Verdict {
	burstAttempts := result.Attempts[min(result.BurstStartIdx, len(result.Attempts)):]
	rejected429 := 0
	otherNonOK := 0
	for _, a := range burstAttempts {
		if a.OK {
			continue
		}
		if a.StatusCode == http.StatusTooManyRequests {
			rejected429++
		} else {
			otherNonOK++
		}
	}
	expectedExcess := p.ExcessBurstAttempts()
	lowerBound := int(float64(expectedExcess) * (1 - rejectionTolerance))
	upperBound := int(float64(expectedExcess) * (1 + rejectionTolerance))
	numbers := map[string]any{
		"burstAttempts":  len(burstAttempts),
		"rejected429":    rejected429,
		"otherNonOK":     otherNonOK,
		"expectedExcess": expectedExcess,
		"lowerBound":     lowerBound,
		"upperBound":     upperBound,
	}

	if rejected429 == 0 {
		return Verdict{
			Clause: "capacity_rejection_defer",
			Status: VerdictFail,
			Detail: fmt.Sprintf("0 of %d burst-window attempts were visibly capacity-rejected (429 Too Many Requests); the admission-control mechanism does not appear to be active. Expected excess above the %.0f/s envelope: %d attempts.",
				len(burstAttempts), p.NormalOfferedRate, expectedExcess),
			Numbers: numbers,
		}
	}

	status := VerdictPass
	detail := fmt.Sprintf("%d of %d burst-window attempts were visibly capacity-rejected (429 Too Many Requests), within the accepted range [%d, %d] (expected excess %d above the %.0f/s envelope, +/-%.0f%%)",
		rejected429, len(burstAttempts), lowerBound, upperBound, expectedExcess, p.NormalOfferedRate, rejectionTolerance*100)
	switch {
	case rejected429 < lowerBound:
		status = VerdictFail
		detail = fmt.Sprintf("only %d of %d burst-window attempts were capacity-rejected (429); expected at least %d (within %.0f%% of the expected excess of %d) above the %.0f/s envelope -- too few rejections, admission control may be under-enforcing",
			rejected429, len(burstAttempts), lowerBound, rejectionTolerance*100, expectedExcess, p.NormalOfferedRate)
	case rejected429 > upperBound:
		status = VerdictFail
		detail = fmt.Sprintf("%d of %d burst-window attempts were capacity-rejected (429); expected at most %d (within %.0f%% of the expected excess of %d) above the %.0f/s envelope -- too many rejections, admission control may be over-enforcing and refusing attempts that were still within the approved envelope",
			rejected429, len(burstAttempts), upperBound, rejectionTolerance*100, expectedExcess, p.NormalOfferedRate)
	}
	if otherNonOK > 0 {
		detail += fmt.Sprintf("; %d additional non-OK, non-429 attempt(s) observed -- inspect StatusCode/Err detail per attempt in the raw report before assuming these are also capacity-related", otherNonOK)
	}
	return Verdict{
		Clause:  "capacity_rejection_defer",
		Status:  status,
		Detail:  detail,
		Numbers: numbers,
	}
}

func ac022NoSilentLoss(result *SustainedRunResult) Verdict {
	var admittedIDs []int64
	for _, a := range result.Attempts {
		if a.OK {
			admittedIDs = append(admittedIDs, a.SubmissionID)
		}
	}
	return Verdict{
		Clause:  "no_silent_loss",
		Status:  VerdictPass, // caller (main.go) replaces this with the real check via ac022VerifyNoLoss once DB access is available at report-assembly time
		Detail:  fmt.Sprintf("%d submissions recorded as admitted by the HTTP response", len(admittedIDs)),
		Numbers: map[string]any{"admittedCount": len(admittedIDs)},
	}
}

// AC022VerifyNoLoss re-derives the no_silent_loss clause against the
// database (SubmissionStatuses), replacing the placeholder
// ac022NoSilentLoss produced without DB access. Split out so
// RunSustainedPhase's own signature does not need to also thread a
// reconciliation dependency through every helper above.
func AC022VerifyNoLoss(ctx context.Context, db *sql.DB, result *SustainedRunResult) (Verdict, error) {
	var admittedIDs []int64
	for _, a := range result.Attempts {
		if a.OK {
			admittedIDs = append(admittedIDs, a.SubmissionID)
		}
	}
	statuses, err := SubmissionStatuses(ctx, db, admittedIDs)
	if err != nil {
		return Verdict{}, wrapEnvErr("verify no silent loss", err)
	}
	var missing []int64
	for _, id := range admittedIDs {
		if _, ok := statuses[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return Verdict{
			Clause:  "no_silent_loss",
			Status:  VerdictFail,
			Detail:  fmt.Sprintf("%d of %d admitted submissions are missing from the submissions table: %v", len(missing), len(admittedIDs), joinLimitedInt64(missing, 20)),
			Numbers: map[string]any{"admittedCount": len(admittedIDs), "missingCount": len(missing)},
		}, nil
	}
	return Verdict{
		Clause:  "no_silent_loss",
		Status:  VerdictPass,
		Detail:  fmt.Sprintf("all %d admitted submissions are present in the submissions table", len(admittedIDs)),
		Numbers: map[string]any{"admittedCount": len(admittedIDs)},
	}, nil
}

func ac022BacklogDrain(result *SustainedRunResult) Verdict {
	if len(result.StatusSeries) == 0 {
		return Verdict{Clause: "backlog_drain", Status: VerdictInformational, Detail: "no status samples were captured"}
	}
	nonTerminal := func(counts map[string]int64) int64 {
		var n int64
		for status, c := range counts {
			if status != "evidenced" {
				n += c
			}
		}
		return n
	}
	maxBacklog := int64(0)
	for _, s := range result.StatusSeries {
		if n := nonTerminal(s.Counts); n > maxBacklog {
			maxBacklog = n
		}
	}
	final := nonTerminal(result.StatusSeries[len(result.StatusSeries)-1].Counts)
	peakByStatus, finalByStatus := backlogByStatus(result.StatusSeries)
	return Verdict{
		Clause: "backlog_drain",
		Status: VerdictInformational,
		Detail: fmt.Sprintf("peak non-terminal backlog observed: %d; non-terminal backlog at the end of the drain wait: %d (no numeric drain-time target is defined by AC-022). Per-status breakdown and the full sample series are preserved in this clause's Numbers field so a stalled or slow-draining stage can be identified precisely.", maxBacklog, final),
		Numbers: map[string]any{
			"peakBacklog":   maxBacklog,
			"finalBacklog":  final,
			"sampleCount":   len(result.StatusSeries),
			"peakByStatus":  peakByStatus,
			"finalByStatus": finalByStatus,
			"series":        result.StatusSeries,
		},
	}
}

// backlogByStatus reduces a StatusSeries to peak and final counts per
// individual non-terminal status -- pulled out of ac022BacklogDrain so
// it's independently unit-testable. peakByStatus tracks each status's
// own high-water mark independently (not necessarily all reached at the
// same sample), matching how maxBacklog above already treats the
// aggregate peak; finalByStatus is just the last sample's per-status
// breakdown. 'evidenced' is excluded from both, consistent with the
// aggregate backlog definition already used by ac022BacklogDrain.
func backlogByStatus(series []StatusSample) (peakByStatus, finalByStatus map[string]int64) {
	peakByStatus = map[string]int64{}
	for _, s := range series {
		for status, c := range s.Counts {
			if status == "evidenced" {
				continue
			}
			if c > peakByStatus[status] {
				peakByStatus[status] = c
			}
		}
	}
	finalByStatus = map[string]int64{}
	if len(series) > 0 {
		for status, c := range series[len(series)-1].Counts {
			if status != "evidenced" {
				finalByStatus[status] = c
			}
		}
	}
	return peakByStatus, finalByStatus
}

func joinLimited(items []string, limit int) string {
	if len(items) <= limit {
		return fmt.Sprintf("%v", items)
	}
	return fmt.Sprintf("%v ... (%d more)", items[:limit], len(items)-limit)
}

func joinLimitedInt64(items []int64, limit int) []int64 {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}
