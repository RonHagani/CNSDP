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

// ac023BoundedGrowthEvidence is always EVIDENCE_ONLY, unconditionally --
// NFR-035 requires consumption to "remain bounded and not grow
// indefinitely" but defines no number, percentage, or trend rule to
// adjudicate against, and this decision does not invent one. Split out
// from ac023Verdict so its EVIDENCE_ONLY status can never be affected by
// any other clause's own logic.
func ac023BoundedGrowthEvidence(samples []ResourceSample) Verdict {
	growthDetail := fmt.Sprintf(
		"%d CPU/memory samples collected across the run. NFR-035 requires consumption to 'remain bounded and not grow indefinitely' but defines no number, percentage, or trend rule to adjudicate against -- classified as evidence only. Raw series is preserved in this clause's Numbers field for future adjudication once a concrete bound is approved.",
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

	return Verdict{
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
	}
}

// ac023TablespacePlacementVerified turns a TablespacePlacementReport into
// a mechanically verifiable clause: the exhaustion-trigger clauses below
// are only trustworthy if every product-owned relation genuinely landed
// in the constrained tablespace beforehand, never assumed.
func ac023TablespacePlacementVerified(report *TablespacePlacementReport) Verdict {
	if report.AllInCapacity {
		return Verdict{
			Clause:  "tablespace_placement_verified",
			Status:  VerdictPass,
			Detail:  fmt.Sprintf("all %d product-owned relations (tables, indexes, TOAST tables) confirmed in %s via catalog metadata query, not assumed", len(report.Relations), capacityTablespace),
			Numbers: map[string]any{"relations": report.Relations},
		}
	}
	return Verdict{
		Clause:  "tablespace_placement_verified",
		Status:  VerdictFail,
		Detail:  fmt.Sprintf("relation placement problems: %v", report.Problems),
		Numbers: map[string]any{"relations": report.Relations, "problems": report.Problems},
	}
}

// ac023ExhaustionClauses turns one DriveToExhaustion run into AC-023's
// clause E, split into four independently reported sub-clauses --
// matching this harness's existing AC-022 convention of never collapsing
// several distinct pass conditions into one opaque boolean -- covering
// exactly the four things AC-023's own text requires at resource
// exhaustion: the documented behavior occurs, it is observable in
// diagnostics, nothing is silently lost, and the platform does not
// silently continue in a corrupted state.
func ac023ExhaustionClauses(out *ExhaustionOutcome) []Verdict {
	trigger := Verdict{
		Clause:  "resource_exhaustion_trigger",
		Status:  VerdictFail,
		Detail:  fmt.Sprintf("resource exhaustion was not observed within %d attempts (admitted %d) -- the fixture capacity may be sized too generously, or the mechanism is not working", out.AttemptsSent, out.AttemptsAdmitted),
		Numbers: map[string]any{"attemptsSent": out.AttemptsSent, "attemptsAdmitted": out.AttemptsAdmitted},
	}
	if out.Exhausted {
		trigger.Status = VerdictPass
		trigger.Detail = fmt.Sprintf("genuine SQLSTATE 53100 (disk_full) resource exhaustion observed via %s after %d attempts (%d admitted) against the size-bounded verify-only fixture tablespace", out.ObservedVia, out.AttemptsSent, out.AttemptsAdmitted)
	}

	diagnostic := Verdict{
		Clause: "resource_exhaustion_diagnostic_classification",
		Status: VerdictFail,
		Detail: "the distinct resource_exhausted classification was not found in the fixture application's structured logs",
	}
	if out.AppLogHasMarker {
		diagnostic.Status = VerdictPass
		diagnostic.Detail = "the fixture application's structured logs contain the distinct error_family=resource_exhausted classification (internal/db.IsResourceExhausted, consumed by internal/worker and internal/intake) -- the condition is observable in diagnostics, not merely an undifferentiated failure"
	}
	diagnostic.Numbers = map[string]any{"appLogHasMarker": out.AppLogHasMarker, "postgresLogHasEnospc": out.PostgresLogHasENOSPC}

	noLoss := Verdict{Clause: "resource_exhaustion_no_silent_loss"}
	var shrank []string
	for _, tbl := range tableOrder {
		if out.RowCountsAfter[tbl] < out.RowCountsBefore[tbl] {
			shrank = append(shrank, fmt.Sprintf("%s: %d -> %d", tbl, out.RowCountsBefore[tbl], out.RowCountsAfter[tbl]))
		}
	}
	if len(shrank) == 0 {
		noLoss.Status = VerdictPass
		noLoss.Detail = "no product table's row count on the fixture decreased across the exhaustion attempt -- the failing write left no partial artifact and no previously committed row disappeared"
	} else {
		noLoss.Status = VerdictFail
		noLoss.Detail = fmt.Sprintf("row count decreased after the exhaustion attempt (should never happen -- no delete path exists): %v", shrank)
	}
	noLoss.Numbers = map[string]any{"before": out.RowCountsBefore, "after": out.RowCountsAfter}

	usable := Verdict{
		Clause:  "resource_exhaustion_platform_usable",
		Status:  VerdictFail,
		Detail:  out.PlatformUsableDetail,
		Numbers: map[string]any{"platformUsableAfter": out.PlatformUsableAfter},
	}
	if out.PlatformUsableAfter {
		usable.Status = VerdictPass
	}

	return []Verdict{trigger, diagnostic, noLoss, usable}
}

// ac023Verdict assembles AC-023's complete result. bounded_growth_evidence
// stays EVIDENCE_ONLY unconditionally; every other clause here is a real,
// mechanically verified PASS/FAIL, so Overall is now derived by the
// normal rollupOverall rule (newACResult) rather than hardcoded --
// EVIDENCE_ONLY clauses never contribute to that rollup either way
// (report.go), so bounded_growth_evidence still cannot silently turn into
// a PASS/FAIL by virtue of the other clauses existing.
func ac023Verdict(samples []ResourceSample, noDelete, retrievable Verdict, placement *TablespacePlacementReport, exhaustion *ExhaustionOutcome) ACResult {
	clauses := []Verdict{ac023BoundedGrowthEvidence(samples), noDelete, retrievable, ac023TablespacePlacementVerified(placement)}
	clauses = append(clauses, ac023ExhaustionClauses(exhaustion)...)
	return newACResult("AC-023", "Deployment-lifetime retention and bounded resource consumption, including at resource exhaustion", clauses)
}
