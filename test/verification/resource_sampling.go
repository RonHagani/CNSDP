package main

import (
	"context"
	"fmt"
	"time"
)

// ResourceSample is one instantaneous CPU/memory reading -- evidence
// only. No caller of this file's functions ever attaches a PASS/FAIL
// verdict to a resource number, per design item 4/6: the approved
// documents define no numeric CPU/memory/growth threshold (confirmed by
// docs/non-functional-requirements.md's own "Matters delegated to
// architecture" list and "Unvalidated assumption #3"), so inventing one
// here would misrepresent the approved requirements rather than verify
// them.
type ResourceSample struct {
	At         time.Time `json:"at"`
	CPUPercent float64   `json:"cpuPercent"`
	MemBytes   uint64    `json:"memBytes"`
}

// StartResourceSampling begins sampling containerID's CPU/memory at
// interval in a background goroutine and returns a stop function that
// cancels sampling and returns every sample collected. A single failed
// `docker stats` read is skipped (transient Docker CLI hiccups do not
// abort the run); this phase never returns a Go error of its own, since
// a partial or even empty sample series is still valid evidence to
// report, not a harness failure.
func StartResourceSampling(ctx context.Context, compose *Compose, containerID string, interval time.Duration) (stop func() []ResourceSample) {
	sampleCtx, cancel := context.WithCancel(ctx)
	samplesCh := make(chan ResourceSample, 8192)
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-sampleCtx.Done():
				return
			case <-ticker.C:
				cpu, mem, err := compose.Stats(sampleCtx, containerID)
				if err != nil {
					continue
				}
				select {
				case samplesCh <- ResourceSample{At: time.Now(), CPUPercent: cpu, MemBytes: mem}:
				default:
				}
			}
		}
	}()

	return func() []ResourceSample {
		cancel()
		<-done
		close(samplesCh)
		var out []ResourceSample
		for s := range samplesCh {
			out = append(out, s)
		}
		return out
	}
}

// ac023Verdict never produces PASS or FAIL -- newEvidenceOnlyACResult
// hardcodes Overall to EVIDENCE_ONLY regardless of clause content, so a
// future clause addition here cannot silently start reporting a
// pass/fail this AC's approved text does not define.
func ac023Verdict(samples []ResourceSample) ACResult {
	growthDetail := fmt.Sprintf(
		"%d CPU/memory samples collected across the run. NFR-035 requires consumption to 'remain bounded and not grow indefinitely' but defines no number, percentage, or trend rule to adjudicate against -- classified as evidence only, per design item 4/6. Raw series is preserved in this clause's Numbers field for future adjudication once a concrete bound is approved.",
		len(samples))

	var minMem, maxMem uint64
	var maxCPU float64
	if len(samples) > 0 {
		minMem, maxMem = samples[0].MemBytes, samples[0].MemBytes
		for _, s := range samples {
			if s.MemBytes < minMem {
				minMem = s.MemBytes
			}
			if s.MemBytes > maxMem {
				maxMem = s.MemBytes
			}
			if s.CPUPercent > maxCPU {
				maxCPU = s.CPUPercent
			}
		}
	}

	clauses := []Verdict{
		{
			Clause: "bounded_growth_evidence",
			Status: VerdictEvidenceOnly,
			Detail: growthDetail,
			Numbers: map[string]any{
				"sampleCount":   len(samples),
				"minMemBytes":   minMem,
				"maxMemBytes":   maxMem,
				"maxCPUPercent": maxCPU,
				"series":        samples,
			},
		},
		{
			Clause: "resource_exhaustion_trigger",
			Status: VerdictEvidenceOnly,
			Detail: "not executable: no documented resource limit exists anywhere in the approved architecture/NFR documents for the platform to deliberately reach (docs/non-functional-requirements.md's own 'Matters delegated to architecture' list names this explicitly, and its 'Unvalidated assumption #3' -- 'a documented resource limit exists by the time AC-023's resource-exhaustion branch is executed' -- is not currently satisfied). Defining a limit now would be an architecture decision this verification slice is not authorized to make.",
		},
	}
	return newEvidenceOnlyACResult("AC-023", "Deployment-lifetime retention and bounded resource consumption, including at resource exhaustion", clauses)
}
