package main

import "testing"

func TestAC023Verdict_BoundedGrowthEvidenceNeverDrivesOverall(t *testing.T) {
	// Every other clause PASS; bounded_growth_evidence stays EVIDENCE_ONLY
	// and must not prevent Overall from reflecting the real clauses.
	noDelete := Verdict{Clause: "no_deletion_or_contradicting_lifecycle", Status: VerdictPass}
	retrievable := Verdict{Clause: "early_artifact_retrievable", Status: VerdictPass}
	placement := &TablespacePlacementReport{AllInCapacity: true, Relations: []TablespaceRelation{{Name: "submissions", Kind: "r", Tablespace: capacityTablespace}}}
	exhaustion := &ExhaustionOutcome{
		Exhausted:           true,
		ObservedVia:         "http_503",
		AppLogHasMarker:     true,
		PlatformUsableAfter: true,
		RowCountsBefore:     map[string]int64{"submissions": 0},
		RowCountsAfter:      map[string]int64{"submissions": 5},
	}

	ac := ac023Verdict(nil, noDelete, retrievable, placement, exhaustion)

	if ac.Overall != VerdictPass {
		t.Errorf("Overall = %s, want PASS (every non-evidence-only clause passed)", ac.Overall)
	}
	var sawEvidenceOnly bool
	for _, c := range ac.Clauses {
		if c.Clause == "bounded_growth_evidence" {
			sawEvidenceOnly = true
			if c.Status != VerdictEvidenceOnly {
				t.Errorf("bounded_growth_evidence status = %s, want EVIDENCE_ONLY", c.Status)
			}
		}
	}
	if !sawEvidenceOnly {
		t.Error("bounded_growth_evidence clause missing")
	}
}

func TestAC023Verdict_ExhaustionFailure_DrivesOverallToFail(t *testing.T) {
	noDelete := Verdict{Clause: "no_deletion_or_contradicting_lifecycle", Status: VerdictPass}
	retrievable := Verdict{Clause: "early_artifact_retrievable", Status: VerdictPass}
	placement := &TablespacePlacementReport{AllInCapacity: true}
	exhaustion := &ExhaustionOutcome{Exhausted: false, RowCountsBefore: map[string]int64{}, RowCountsAfter: map[string]int64{}}

	ac := ac023Verdict(nil, noDelete, retrievable, placement, exhaustion)

	if ac.Overall != VerdictFail {
		t.Errorf("Overall = %s, want FAIL (exhaustion never observed)", ac.Overall)
	}
}

func TestAC023TablespacePlacementVerified_AllInCapacity_Passes(t *testing.T) {
	report := &TablespacePlacementReport{
		AllInCapacity: true,
		Relations:     []TablespaceRelation{{Name: "submissions", Kind: "r", Tablespace: capacityTablespace}},
	}
	v := ac023TablespacePlacementVerified(report)
	if v.Status != VerdictPass {
		t.Errorf("Status = %s, want PASS", v.Status)
	}
}

func TestAC023TablespacePlacementVerified_RelationLeftInPgDefault_Fails(t *testing.T) {
	report := &TablespacePlacementReport{
		AllInCapacity: false,
		Relations:     []TablespaceRelation{{Name: "submissions", Kind: "r", Tablespace: "pg_default"}},
		Problems:      []string{`submissions (table) is in tablespace "pg_default", want "verify_capacity"`},
	}
	v := ac023TablespacePlacementVerified(report)
	if v.Status != VerdictFail {
		t.Errorf("Status = %s, want FAIL", v.Status)
	}
}

func TestAC023ExhaustionClauses_NotObserved_AllRelevantClausesFail(t *testing.T) {
	out := &ExhaustionOutcome{
		AttemptsSent:     100,
		AttemptsAdmitted: 100,
		Exhausted:        false,
		RowCountsBefore:  map[string]int64{"submissions": 0},
		RowCountsAfter:   map[string]int64{"submissions": 100},
	}
	clauses := ac023ExhaustionClauses(out)
	byName := clauseMap(clauses)

	if byName["resource_exhaustion_trigger"].Status != VerdictFail {
		t.Errorf("resource_exhaustion_trigger = %s, want FAIL", byName["resource_exhaustion_trigger"].Status)
	}
	if byName["resource_exhaustion_diagnostic_classification"].Status != VerdictFail {
		t.Errorf("resource_exhaustion_diagnostic_classification = %s, want FAIL", byName["resource_exhaustion_diagnostic_classification"].Status)
	}
	// no_silent_loss and platform_usable can legitimately still pass even
	// when exhaustion was never triggered (nothing was lost, nothing broke) --
	// only trigger/diagnostic must fail in this scenario.
	if byName["resource_exhaustion_no_silent_loss"].Status != VerdictPass {
		t.Errorf("resource_exhaustion_no_silent_loss = %s, want PASS (counts only grew)", byName["resource_exhaustion_no_silent_loss"].Status)
	}
}

func TestAC023ExhaustionClauses_Observed_AllPass(t *testing.T) {
	out := &ExhaustionOutcome{
		AttemptsSent:         200,
		AttemptsAdmitted:     199,
		Exhausted:            true,
		ObservedVia:          "app_log",
		AppLogHasMarker:      true,
		PlatformUsableAfter:  true,
		PlatformUsableDetail: "SELECT 1 succeeded and pg_is_in_recovery() = false after the exhaustion attempt",
		RowCountsBefore:      map[string]int64{"submissions": 0, "validation_outcomes": 0},
		RowCountsAfter:       map[string]int64{"submissions": 199, "validation_outcomes": 150},
	}
	clauses := ac023ExhaustionClauses(out)
	for _, c := range clauses {
		if c.Status != VerdictPass {
			t.Errorf("clause %s = %s, want PASS: %s", c.Clause, c.Status, c.Detail)
		}
	}
}

func TestAC023ExhaustionClauses_RowCountDecreased_NoSilentLossFails(t *testing.T) {
	out := &ExhaustionOutcome{
		Exhausted:           true,
		AppLogHasMarker:     true,
		PlatformUsableAfter: true,
		RowCountsBefore:     map[string]int64{"submissions": 50},
		RowCountsAfter:      map[string]int64{"submissions": 40}, // impossible in a real run -- proves the check actually fires
	}
	clauses := ac023ExhaustionClauses(out)
	byName := clauseMap(clauses)
	if byName["resource_exhaustion_no_silent_loss"].Status != VerdictFail {
		t.Errorf("resource_exhaustion_no_silent_loss = %s, want FAIL when a row count decreased", byName["resource_exhaustion_no_silent_loss"].Status)
	}
}

func TestAC023ExhaustionClauses_PlatformUnusableAfter_Fails(t *testing.T) {
	out := &ExhaustionOutcome{
		Exhausted:            true,
		AppLogHasMarker:      true,
		PlatformUsableAfter:  false,
		PlatformUsableDetail: "select-1 err=connection refused",
		RowCountsBefore:      map[string]int64{},
		RowCountsAfter:       map[string]int64{},
	}
	clauses := ac023ExhaustionClauses(out)
	byName := clauseMap(clauses)
	if byName["resource_exhaustion_platform_usable"].Status != VerdictFail {
		t.Errorf("resource_exhaustion_platform_usable = %s, want FAIL", byName["resource_exhaustion_platform_usable"].Status)
	}
}

func clauseMap(clauses []Verdict) map[string]Verdict {
	out := make(map[string]Verdict, len(clauses))
	for _, c := range clauses {
		out[c.Clause] = c
	}
	return out
}

// TestNoDeleteStaticCheck_RealRepo_CurrentlyClean is a regression guard
// against the real, current repository -- supplementary evidence only
// (per design; VerifyRetentionLate's runtime before/after comparison is
// the primary proof), so this test documents today's static state rather
// than gating anything.
func TestNoDeleteStaticCheck_RealRepo_CurrentlyClean(t *testing.T) {
	root, err := repoRootDir()
	if err != nil {
		t.Fatalf("repoRootDir: %v", err)
	}
	clean, detail := noDeleteStaticCheck(root)
	if !clean {
		t.Errorf("noDeleteStaticCheck reports not clean: %s", detail)
	}
}
