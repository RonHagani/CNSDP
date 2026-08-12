package main

import "time"

// Profile selects the harness's execution scale. Smoke validates plumbing
// (compose lifecycle, fixture generation, correlation, reconciliation,
// report emission, phase sequencing) at trivial scale and NEVER produces an
// AC-019 through AC-023 verdict. Full runs the exact approved
// acceptance-criteria parameters and is the only profile whose report may
// be cited as verification evidence -- report.go enforces this in code
// (buildACVerdicts), not only by convention.
type Profile string

const (
	ProfileSmoke Profile = "smoke"
	ProfileFull  Profile = "full"
)

// Params is one profile's concrete workload shape. Every Full value that
// materially affects an AC verdict is taken directly from the approved
// acceptance-criteria text (AC-019 through AC-023); Smoke's values exist
// only to exercise the same code paths quickly.
type Params struct {
	Profile Profile

	// Sustained-run shape (AC-020, AC-022). The overload window is a
	// sub-interval of the total run, not appended after it: total run
	// duration = BaselineDuration + BurstDuration. BurstOfferedRate is the
	// TOTAL offered rate during the window (not additive to
	// NormalOfferedRate) -- resolved from AC-022's exact wording ("a
	// superimposed overload window offering 15 submission attempts per
	// second for 30 seconds during the run"), not the more aggressive
	// additive reading.
	BaselineDuration       time.Duration
	NormalOfferedRate      float64 // attempts/second during the baseline portion
	BurstDuration          time.Duration
	BurstOfferedRate       float64       // TOTAL attempts/second during the overload window
	MinMatchingPerScenario int           // scenario-fixture submissions to seed per scenario (AC-020 requires >=20; Full uses a buffer)
	RateBucketWidth        time.Duration // admitted-rate measurement bucket width (AC-022's per-window envelope check)

	// Retrieval-latency sampling (AC-021).
	RetrievalSamplesPerClassPerCondition int
	IdleWindow                           time.Duration // harness-side silence defining "idle or reset condition" (AC-021 does not define this numerically)

	// Bounded wait, after the sustained-run phase ends, for its backlog to
	// drain before the AC-020 alert-correlation query runs.
	PostRunDrainWait time.Duration

	// Recovery (AC-019).
	RecoveryBatchSize int
	RecoveryRTOBudget time.Duration // 15 minutes, per AC-019 -- unchanged between profiles: it is a ceiling, not a target
	RecoveryDrainWait time.Duration // bounded wait for post-recovery drain to completion

	// Resource sampling (AC-023) -- evidence collection only, no threshold.
	ResourceSampleInterval time.Duration

	// Overall watchdog: cancels every phase and triggers cleanup if exceeded.
	OverallTimeout time.Duration
}

func (p Params) TotalRunDuration() time.Duration {
	return p.BaselineDuration + p.BurstDuration
}

// TotalOfferedNormal/TotalOfferedBurst/TotalOffered report the expected
// offered-attempt counts implied by the configured rates and durations --
// for Full, these must equal exactly 8700 / 450 / 9150 (the approved
// AC-022 arithmetic); asserted by profile_test.go so a future edit to
// FullParams cannot silently drift from the approved numbers.
func (p Params) TotalOfferedNormal() int {
	return int(p.BaselineDuration.Seconds() * p.NormalOfferedRate)
}

func (p Params) TotalOfferedBurst() int {
	return int(p.BurstDuration.Seconds() * p.BurstOfferedRate)
}

func (p Params) TotalOffered() int {
	return p.TotalOfferedNormal() + p.TotalOfferedBurst()
}

// ExcessBurstAttempts is the count above the supported (NormalOfferedRate)
// envelope during the burst window -- what AC-022's "excess attempts...
// visibly rejected or deferred for capacity" pass clause refers to.
func (p Params) ExcessBurstAttempts() int {
	return int(p.BurstDuration.Seconds() * (p.BurstOfferedRate - p.NormalOfferedRate))
}

// FullParams is the authoritative AC-019 through AC-023 verification run.
// Every value here traces to the approved acceptance-criteria document;
// changing one changes what is being verified, not just how fast the
// harness runs -- see docs/acceptance-criteria.md AC-019 through AC-023.
func FullParams() Params {
	return Params{
		Profile: ProfileFull,

		// AC-022 Context: "A 15-minute sustained run at 10 admitted
		// submissions per second, with a superimposed overload window
		// offering 15 submission attempts per second for 30 seconds during
		// the run." 900s total = 870s baseline + 30s burst (sub-interval,
		// not appended). Burst rate is the window's TOTAL offered rate.
		BaselineDuration:  870 * time.Second,
		NormalOfferedRate: 10.0,
		BurstDuration:     30 * time.Second,
		BurstOfferedRate:  15.0,

		// AC-020 Context: ">=20 matching submissions per approved detection
		// scenario, >=60 total". +5 buffer per scenario absorbs any
		// submission that fails validation for an unrelated reason without
		// re-running the full sustained window.
		MinMatchingPerScenario: 25,
		RateBucketWidth:        10 * time.Second,

		PostRunDrainWait: 3 * time.Minute,

		// AC-021 Context: "each sampled at least 10 times", both idle- and
		// active-condition.
		RetrievalSamplesPerClassPerCondition: 10,
		IdleWindow:                           60 * time.Second,

		RecoveryBatchSize: 30,
		RecoveryRTOBudget: 15 * time.Minute, // AC-019's approved RTO
		RecoveryDrainWait: 2 * time.Minute,

		ResourceSampleInterval: 30 * time.Second,

		// Generous ceiling: 15min sustained + 3min drain + ~1min retrieval
		// + up to 15min RTO budget + 2min recovery drain + first-time
		// Docker image build time, all counted against this one watchdog.
		OverallTimeout: 45 * time.Minute,
	}
}

// SmokeParams validates harness plumbing only -- these numbers carry no
// evidentiary meaning and report.go refuses to attach an AC-019 through
// AC-023 verdict to a report produced with this profile.
func SmokeParams() Params {
	return Params{
		Profile: ProfileSmoke,

		BaselineDuration:  40 * time.Second,
		NormalOfferedRate: 5.0,
		BurstDuration:     5 * time.Second,
		BurstOfferedRate:  8.0,

		MinMatchingPerScenario: 2,
		RateBucketWidth:        5 * time.Second,

		PostRunDrainWait: 20 * time.Second,

		RetrievalSamplesPerClassPerCondition: 2,
		IdleWindow:                           3 * time.Second,

		RecoveryBatchSize: 5,
		RecoveryRTOBudget: 15 * time.Minute, // ceiling, not a target -- real recovery is expected in seconds
		RecoveryDrainWait: 30 * time.Second,

		ResourceSampleInterval: 5 * time.Second,

		// Generous enough to include a first-time Docker image build.
		OverallTimeout: 12 * time.Minute,
	}
}

func ParamsFor(p Profile) (Params, error) {
	switch p {
	case ProfileFull:
		return FullParams(), nil
	case ProfileSmoke:
		return SmokeParams(), nil
	default:
		return Params{}, &EnvironmentError{Op: "profile selection", Err: errUnknownProfile(p)}
	}
}

type errUnknownProfile string

func (e errUnknownProfile) Error() string {
	return "unknown profile " + string(e) + " (want \"smoke\" or \"full\")"
}
