package main

import "testing"

func TestRollupOverall(t *testing.T) {
	cases := []struct {
		name    string
		clauses []Verdict
		want    VerdictStatus
	}{
		{"all pass", []Verdict{{Status: VerdictPass}, {Status: VerdictPass}}, VerdictPass},
		{"one fail wins", []Verdict{{Status: VerdictPass}, {Status: VerdictFail}}, VerdictFail},
		{"not_implemented beats pass", []Verdict{{Status: VerdictPass}, {Status: VerdictNotImplemented}}, VerdictNotImplemented},
		{"fail beats not_implemented", []Verdict{{Status: VerdictNotImplemented}, {Status: VerdictFail}}, VerdictFail},
		{"only informational", []Verdict{{Status: VerdictInformational}}, VerdictInformational},
		{"only evidence_only never becomes pass", []Verdict{{Status: VerdictEvidenceOnly}}, VerdictInformational},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rollupOverall(c.clauses); got != c.want {
				t.Errorf("rollupOverall(%v) = %s, want %s", c.clauses, got, c.want)
			}
		})
	}
}

func TestSetACResults_SmokeProfile_NeverEmitsACVerdicts(t *testing.T) {
	r := NewReport(ProfileSmoke, "test-run")
	r.SetACResults([]ACResult{
		newACResult("AC-020", "t", []Verdict{{Status: VerdictFail}}),
	})
	if r.ACResults != nil {
		t.Fatalf("smoke report.ACResults = %v, want nil -- a smoke run must never emit an authoritative AC verdict", r.ACResults)
	}
	found := false
	for _, n := range r.Notes {
		if n != "" {
			found = true
		}
	}
	if !found {
		t.Error("expected a note explaining the discarded AC results")
	}
}

func TestSetACResults_FullProfile_KeepsACVerdicts(t *testing.T) {
	r := NewReport(ProfileFull, "test-run")
	results := []ACResult{newACResult("AC-020", "t", []Verdict{{Status: VerdictPass}})}
	r.SetACResults(results)
	if len(r.ACResults) != 1 {
		t.Fatalf("full report.ACResults = %v, want the one result passed in", r.ACResults)
	}
}

func TestExitCode_EnvironmentErrorAlwaysWins(t *testing.T) {
	r := NewReport(ProfileFull, "test-run")
	r.Phases = []PhaseHealth{{Phase: "sustained_run", Succeeded: false, Error: "docker exploded"}}
	r.SetACResults([]ACResult{newACResult("AC-020", "t", []Verdict{{Status: VerdictPass}})})
	if got := r.ExitCode(); got != exitEnvironmentError {
		t.Errorf("ExitCode() = %d, want exitEnvironmentError (%d) -- an environment failure must win over any AC verdict", got, exitEnvironmentError)
	}
}

func TestExitCode_RequirementFailure(t *testing.T) {
	r := NewReport(ProfileFull, "test-run")
	r.Phases = []PhaseHealth{{Phase: "sustained_run", Succeeded: true}}
	r.SetACResults([]ACResult{newACResult("AC-022", "t", []Verdict{{Status: VerdictNotImplemented}})})
	if got := r.ExitCode(); got != exitRequirementFailed {
		t.Errorf("ExitCode() = %d, want exitRequirementFailed (%d)", got, exitRequirementFailed)
	}
}

func TestExitCode_SmokeCannotProduceRequirementFailure(t *testing.T) {
	r := NewReport(ProfileSmoke, "test-run")
	r.Phases = []PhaseHealth{{Phase: "sustained_run", Succeeded: true}}
	r.SetACResults([]ACResult{newACResult("AC-022", "t", []Verdict{{Status: VerdictFail}})}) // discarded by SetACResults
	if got := r.ExitCode(); got != exitOK {
		t.Errorf("ExitCode() = %d, want exitOK (%d) -- smoke's discarded AC results must never drive exitRequirementFailed", got, exitOK)
	}
}

func TestExitCode_AllGreen(t *testing.T) {
	r := NewReport(ProfileFull, "test-run")
	r.Phases = []PhaseHealth{{Phase: "sustained_run", Succeeded: true}}
	r.SetACResults([]ACResult{newACResult("AC-020", "t", []Verdict{{Status: VerdictPass}})})
	if got := r.ExitCode(); got != exitOK {
		t.Errorf("ExitCode() = %d, want exitOK (%d)", got, exitOK)
	}
}
