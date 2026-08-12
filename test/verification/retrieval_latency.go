package main

import (
	"context"
	"fmt"
	"sort"
	"time"
)

// retrievalSample is one timed GET, tagged with which condition (idle or
// active) it represents.
type retrievalSample struct {
	Condition  string // "idle" or "active"
	Elapsed    time.Duration
	StatusCode int
	Err        string
}

// RetrievalLatencyResult groups samples by artifact class, matching
// AC-021's Context exactly: "validation outcomes, alerts, and
// minimum-evidence-set artifacts" -- GET /v1/detections and
// GET /v1/data-sources are not named there and are deliberately excluded.
type RetrievalLatencyResult struct {
	ByClass map[string][]retrievalSample
}

// RunRetrievalLatencyPhase implements AC-021. Interpretation decision
// (disclosed, not silent): AC-021's Context reads "each sampled at least
// 10 times, including one first retrieval following a documented idle or
// reset condition and repeated retrievals under an already-active
// condition" -- read here as >=10 samples per class TOTAL, of which
// exactly one is the idle-following sample and the remainder are active,
// not >=10 of EACH condition per class. The stricter (>=10-per-condition)
// reading was considered during design but rejected during
// implementation: at Full's 60s idle window, three independent
// >=10-sample idle waits per class would cost >=30 minutes of pure wait
// time for this phase alone, which is disproportionate to what the text
// actually requires and to the rest of the run's budget. One combined
// idle window (IdleWindow) is observed once, at the start of this phase,
// and the first sample of every class is taken immediately after it --
// each is genuinely "the first retrieval of that class following the
// idle condition" -- before proceeding to active-condition repeats.
func RunRetrievalLatencyPhase(ctx context.Context, api *APIClient, p Params, alertIDs []int64) (*RetrievalLatencyResult, error) {
	result := &RetrievalLatencyResult{ByClass: map[string][]retrievalSample{}}

	sampleOnce := func(class, path, condition string) {
		elapsed, status, err := api.TimedGet(ctx, path)
		s := retrievalSample{Condition: condition, Elapsed: elapsed, StatusCode: status}
		if err != nil {
			s.Err = err.Error()
		}
		result.ByClass[class] = append(result.ByClass[class], s)
	}

	// Observe one shared idle window (platform-wide silence), then take
	// the idle-following sample for every class in immediate succession.
	sleepUntil(ctx, time.Now().Add(p.IdleWindow))

	sampleOnce("validation_outcomes", "/v1/submissions", "idle")
	sampleOnce("alerts", "/v1/alerts", "idle")
	if len(alertIDs) > 0 {
		sampleOnce("minimum_evidence_set", fmt.Sprintf("/v1/alerts/%d", alertIDs[0]), "idle")
	}

	activeNeeded := p.RetrievalSamplesPerClassPerCondition - 1 // one already taken (idle)
	if activeNeeded < 0 {
		activeNeeded = 0
	}

	for i := 0; i < activeNeeded; i++ {
		sampleOnce("validation_outcomes", "/v1/submissions", "active")
	}
	for i := 0; i < activeNeeded; i++ {
		sampleOnce("alerts", "/v1/alerts", "active")
	}
	for i := 0; i < activeNeeded; i++ {
		id := alertIDs[i%len(alertIDsOrPlaceholder(alertIDs))]
		sampleOnce("minimum_evidence_set", fmt.Sprintf("/v1/alerts/%d", id), "active")
	}

	return result, nil
}

func alertIDsOrPlaceholder(ids []int64) []int64 {
	if len(ids) == 0 {
		return []int64{0}
	}
	return ids
}

// ac021Verdict applies AC-021's all-samples rule (<=5s, every sample,
// every class, both conditions) to the collected samples.
func ac021Verdict(result *RetrievalLatencyResult) ACResult {
	var clauses []Verdict
	classNames := make([]string, 0, len(result.ByClass))
	for class := range result.ByClass {
		classNames = append(classNames, class)
	}
	sort.Strings(classNames)

	for _, class := range classNames {
		samples := result.ByClass[class]
		if len(samples) == 0 {
			clauses = append(clauses, Verdict{
				Clause: class,
				Status: VerdictFail,
				Detail: "no samples were collected for this artifact class",
			})
			continue
		}

		durations := make([]time.Duration, 0, len(samples))
		var maxD time.Duration
		var failures int
		for _, s := range samples {
			durations = append(durations, s.Elapsed)
			if s.Elapsed > maxD {
				maxD = s.Elapsed
			}
			if s.Err != "" || s.StatusCode != 200 || s.Elapsed > 5*time.Second {
				failures++
			}
		}
		sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })

		status := VerdictPass
		detail := fmt.Sprintf("%d samples, max %s, all within the 5s target", len(samples), maxD)
		if failures > 0 {
			status = VerdictFail
			detail = fmt.Sprintf("%d of %d samples violated the 5s all-samples target or returned an error/non-200 status (max observed %s)", failures, len(samples), maxD)
		}

		clauses = append(clauses, Verdict{
			Clause: class,
			Status: status,
			Detail: detail,
			Numbers: map[string]any{
				"count": len(durations),
				"min":   durations[0].String(),
				"p50":   percentile(durations, 50).String(),
				"p95":   percentile(durations, 95).String(),
				"p99":   percentile(durations, 99).String(),
				"max":   durations[len(durations)-1].String(),
			},
		})
	}

	return newACResult("AC-021", "Retrieval responsiveness of 5 seconds or less", clauses)
}
