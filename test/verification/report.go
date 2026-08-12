package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Design rule followed by every phase file: a Go `error` returned from a
// phase function means the HARNESS could not execute correctly
// (EnvironmentError) and aborts the run -- it is never how a measured
// requirement violation is reported. A measured AC-019 through AC-023
// threshold violation is always a normal, successful return value: an
// ACResult with Overall == VerdictFail (or VerdictNotImplemented).
// Conflating the two would make a flaky Docker hiccup indistinguishable
// from a genuine product regression, which is exactly the distinction
// item 9/D of the design requires.

type VerdictStatus string

const (
	VerdictPass           VerdictStatus = "PASS"
	VerdictFail           VerdictStatus = "FAIL"
	VerdictNotImplemented VerdictStatus = "NOT_IMPLEMENTED"
	VerdictEvidenceOnly   VerdictStatus = "EVIDENCE_ONLY"
	VerdictNotEvaluated   VerdictStatus = "NOT_EVALUATED"
	VerdictInformational  VerdictStatus = "INFORMATIONAL"
)

// Verdict is one independently reported clause of an AC -- AC-022 in
// particular must never collapse "admitted-rate envelope",
// "offered/admitted counts", "capacity rejection/defer",
// "no silent loss", and "backlog/drain" into one opaque boolean; each is
// its own Verdict inside that AC's Clauses.
type Verdict struct {
	Clause  string         `json:"clause"`
	Status  VerdictStatus  `json:"status"`
	Detail  string         `json:"detail"`
	Numbers map[string]any `json:"numbers,omitempty"`
}

// ACResult is one acceptance criterion's full, itemized result. Overall
// is derived from Clauses by rollupOverall -- never set independently of
// them, so the top-level status can never silently disagree with the
// detail a reader would see just below it.
type ACResult struct {
	AC      string        `json:"ac"`
	Title   string        `json:"title"`
	Overall VerdictStatus `json:"overall"`
	Clauses []Verdict     `json:"clauses"`
}

// rollupOverall computes an AC's overall status from its clauses:
// PASS/FAIL/NOT_IMPLEMENTED clauses drive it (worst-first: FAIL beats
// NOT_IMPLEMENTED beats PASS); INFORMATIONAL and EVIDENCE_ONLY clauses
// never contribute to it either way, so an AC whose only substantive
// clauses are informational is never silently marked PASS by omission --
// callers that want that shape set forceEvidenceOnly instead (AC-023).
func rollupOverall(clauses []Verdict) VerdictStatus {
	sawFail, sawNotImpl, sawPass := false, false, false
	for _, c := range clauses {
		switch c.Status {
		case VerdictFail:
			sawFail = true
		case VerdictNotImplemented:
			sawNotImpl = true
		case VerdictPass:
			sawPass = true
		}
	}
	switch {
	case sawFail:
		return VerdictFail
	case sawNotImpl:
		return VerdictNotImplemented
	case sawPass:
		return VerdictPass
	default:
		return VerdictInformational
	}
}

func newACResult(ac, title string, clauses []Verdict) ACResult {
	return ACResult{AC: ac, Title: title, Overall: rollupOverall(clauses), Clauses: clauses}
}

// newEvidenceOnlyACResult is for AC-023: no clause here is ever
// PASS/FAIL, by design (item 6/4 of the corrected design) -- Overall is
// hardcoded to EVIDENCE_ONLY regardless of clause content, never derived,
// so a future clause addition cannot accidentally make this AC report
// PASS/FAIL without a deliberate code change to this function itself.
func newEvidenceOnlyACResult(ac, title string, clauses []Verdict) ACResult {
	return ACResult{AC: ac, Title: title, Overall: VerdictEvidenceOnly, Clauses: clauses}
}

// PhaseHealth records whether one harness phase executed to completion
// without an environment-level failure -- independent of what its AC
// verdicts came out as. A phase that ran fully and found a genuine
// requirement violation is Succeeded == true with a FAIL verdict attached
// elsewhere; Succeeded == false means the harness itself broke.
type PhaseHealth struct {
	Phase     string    `json:"phase"`
	Succeeded bool      `json:"succeeded"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"startedAt"`
	Duration  string    `json:"duration"`
}

// Report is the complete output of one harness run.
type Report struct {
	Profile        Profile          `json:"profile"`
	RunID          string           `json:"runId"`
	GeneratedAt    time.Time        `json:"generatedAt"`
	Host           string           `json:"host"`
	GoVersion      string           `json:"goVersion"`
	DockerVersion  string           `json:"dockerVersion,omitempty"`
	ComposeVersion string           `json:"composeVersion,omitempty"`
	Isolation      *IsolationReport `json:"isolation,omitempty"`
	Phases         []PhaseHealth    `json:"phases"`
	ACResults      []ACResult       `json:"acResults,omitempty"`
	Notes          []string         `json:"notes,omitempty"`
}

// NewReport starts a report for profile, stamping run identity and
// execution context so a captured number is always presented as
// "measured under these conditions, on this date" rather than asserted as
// a universal guarantee (design item 10).
func NewReport(profile Profile, runID string) *Report {
	host, _ := os.Hostname()
	return &Report{
		Profile:     profile,
		RunID:       runID,
		GeneratedAt: time.Now().UTC(),
		Host:        fmt.Sprintf("%s (%s/%s)", host, runtime.GOOS, runtime.GOARCH),
		GoVersion:   runtime.Version(),
	}
}

// SetACResults is the single, hard-coded enforcement point for
// "a smoke report must never emit authoritative PASS/FAIL AC-019 through
// AC-023 verdicts" (design item 7): it refuses to attach any AC result
// when Profile == ProfileSmoke, regardless of what the caller passes,
// converting the attempt into a note instead of an exported verdict. Full
// profile is unaffected. main.go must call this rather than assigning
// r.ACResults directly, so the guarantee cannot be bypassed by a call
// site that forgets to check the profile itself.
func (r *Report) SetACResults(results []ACResult) {
	if r.Profile == ProfileSmoke {
		if len(results) > 0 {
			r.Notes = append(r.Notes, fmt.Sprintf(
				"smoke profile: %d AC verdict(s) computed internally were discarded, not reported -- smoke runs never emit authoritative AC-019 through AC-023 verdicts",
				len(results)))
		}
		r.ACResults = nil
		return
	}
	r.ACResults = results
}

// ExitCode implements the environment-error-vs-requirement-failure exit
// taxonomy (errors.go): any phase that did not succeed means the harness
// itself broke, and that always wins over any AC verdict, since an AC
// verdict computed after an environment failure is not trustworthy. Only
// once every phase succeeded does a full-profile FAIL or NOT_IMPLEMENTED
// AC verdict drive exitRequirementFailed. Smoke never reaches the
// requirement-failure branch: SetACResults guarantees r.ACResults is
// empty for smoke.
func (r *Report) ExitCode() int {
	for _, p := range r.Phases {
		if !p.Succeeded {
			return exitEnvironmentError
		}
	}
	for _, ac := range r.ACResults {
		if ac.Overall == VerdictFail || ac.Overall == VerdictNotImplemented {
			return exitRequirementFailed
		}
	}
	return exitOK
}

// WriteJSON writes the report to dir under a timestamped, run-id-bearing
// filename and returns the path.
func (r *Report) WriteJSON(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create results directory: %w", err)
	}
	name := fmt.Sprintf("%s-%s-%s.json", r.GeneratedAt.Format("20060102T150405Z"), r.Profile, r.RunID)
	path := filepath.Join(dir, name)

	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return "", fmt.Errorf("write report file: %w", err)
	}
	return path, nil
}

// HumanSummary renders a readable summary. Every AC line is prefixed
// [FULL] or, for a smoke run (where r.ACResults is always empty per
// SetACResults), the summary shows phase health only and says so
// explicitly -- never a bare AC-019..023 status line that a reader could
// mistake for evidence.
func (r *Report) HumanSummary() string {
	var b strings.Builder
	fmt.Fprintf(&b, "CNSDP verification run %s\n", r.RunID)
	fmt.Fprintf(&b, "profile: %s | generated: %s | host: %s\n", r.Profile, r.GeneratedAt.Format(time.RFC3339), r.Host)
	if r.Isolation != nil {
		fmt.Fprintf(&b, "compose isolation: satisfied=%v (project=%s)\n", r.Isolation.Satisfied, r.Isolation.ProjectName)
	}
	b.WriteString("\nphase health:\n")
	for _, p := range r.Phases {
		status := "ok"
		if !p.Succeeded {
			status = "ENVIRONMENT ERROR: " + p.Error
		}
		fmt.Fprintf(&b, "  - %-24s %-8s %s\n", p.Phase, p.Duration, status)
	}

	if r.Profile == ProfileSmoke {
		b.WriteString("\nsmoke profile: no AC-019 through AC-023 verdicts are reported by design.\n")
		b.WriteString("this report only certifies harness plumbing (above); it is not verification evidence.\n")
	} else {
		b.WriteString("\nAC verdicts (full profile -- authoritative):\n")
		acs := append([]ACResult(nil), r.ACResults...)
		sort.Slice(acs, func(i, j int) bool { return acs[i].AC < acs[j].AC })
		for _, ac := range acs {
			fmt.Fprintf(&b, "  %s %-6s %s\n", ac.AC, ac.Overall, ac.Title)
			for _, cl := range ac.Clauses {
				fmt.Fprintf(&b, "      - %-28s %-16s %s\n", cl.Clause, cl.Status, cl.Detail)
			}
		}
	}

	if len(r.Notes) > 0 {
		b.WriteString("\nnotes:\n")
		for _, n := range r.Notes {
			fmt.Fprintf(&b, "  - %s\n", n)
		}
	}

	fmt.Fprintf(&b, "\nexit code: %d\n", r.ExitCode())
	return b.String()
}
