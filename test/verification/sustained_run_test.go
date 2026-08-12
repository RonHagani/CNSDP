package main

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildWorkloadPlan_ExactPerScenarioCounts(t *testing.T) {
	f := loadTestFixtures(t)
	p := SmokeParams() // small enough to run fast; exercises the same code path as Full

	plan, err := buildWorkloadPlan(f, p)
	if err != nil {
		t.Fatalf("buildWorkloadPlan: %v", err)
	}
	if len(plan) != p.TotalOffered() {
		t.Fatalf("len(plan) = %d, want %d", len(plan), p.TotalOffered())
	}

	counts := map[ScenarioID]int{}
	for _, item := range plan {
		if item.Scenario != "" {
			counts[item.Scenario]++
		}
		if len(item.Item) == 0 {
			t.Fatal("plan contains an empty item")
		}
	}
	for _, sid := range []ScenarioID{Scenario1, Scenario2, Scenario3} {
		if counts[sid] != p.MinMatchingPerScenario {
			t.Errorf("scenario %s: got %d, want exactly %d", sid, counts[sid], p.MinMatchingPerScenario)
		}
	}
}

func TestBuildWorkloadPlan_AllAuditIDsUnique(t *testing.T) {
	f := loadTestFixtures(t)
	p := SmokeParams()

	plan, err := buildWorkloadPlan(f, p)
	if err != nil {
		t.Fatalf("buildWorkloadPlan: %v", err)
	}

	seen := map[string]bool{}
	for i, item := range plan {
		var m map[string]any
		if err := json.Unmarshal(item.Item, &m); err != nil {
			t.Fatalf("item %d: unmarshal: %v", i, err)
		}
		id, _ := m["auditID"].(string)
		if id == "" {
			t.Fatalf("item %d has an empty auditID", i)
		}
		if seen[id] {
			t.Fatalf("item %d: duplicate auditID %s", i, id)
		}
		seen[id] = true
	}
}

func TestAC022CapacityRejectionDefer_AllAdmitted_ReportsNotImplemented(t *testing.T) {
	p := FullParams()
	result := &SustainedRunResult{
		Attempts:      makeAllOKAttempts(10),
		BurstStartIdx: 5,
	}
	v := ac022CapacityRejectionDefer(p, result)
	if v.Status != VerdictNotImplemented {
		t.Errorf("Status = %s, want %s when every burst attempt was admitted", v.Status, VerdictNotImplemented)
	}
}

func TestAC022CapacityRejectionDefer_SomeRejected_NotNotImplemented(t *testing.T) {
	p := FullParams()
	attempts := makeAllOKAttempts(10)
	attempts[6].OK = false
	result := &SustainedRunResult{Attempts: attempts, BurstStartIdx: 5}
	v := ac022CapacityRejectionDefer(p, result)
	if v.Status == VerdictNotImplemented {
		t.Errorf("Status should not be %s once at least one burst attempt was rejected", VerdictNotImplemented)
	}
}

func TestAC020Verdict_AllSamplesRule_MaxOverThresholdFails(t *testing.T) {
	now := time.Now()
	admitted := now
	alertAt := now.Add(61 * time.Second) // over the 60s target
	attempts := []Attempt{{SubmissionID: 1, Scenario: Scenario1, OK: true}}
	result := &SustainedRunResult{
		Attempts: attempts,
		Correlations: map[int64]AlertCorrelation{
			1: {SubmissionID: 1, AdmittedAt: admitted, AlertID: int64Ptr(1), AlertAt: timePtr(alertAt)},
		},
	}
	ac := ac020Verdict(result)
	if ac.Overall != VerdictFail {
		t.Errorf("Overall = %s, want %s for a 61s latency sample", ac.Overall, VerdictFail)
	}
}

func TestAC020Verdict_UnresolvedSubmission_Fails(t *testing.T) {
	attempts := []Attempt{{SubmissionID: 1, Scenario: Scenario1, OK: true}}
	result := &SustainedRunResult{Attempts: attempts, Correlations: map[int64]AlertCorrelation{}}
	ac := ac020Verdict(result)
	if ac.Overall != VerdictFail {
		t.Errorf("Overall = %s, want %s for a submission that never resolved to an alert", ac.Overall, VerdictFail)
	}
}

func makeAllOKAttempts(n int) []Attempt {
	out := make([]Attempt, n)
	base := time.Now()
	for i := range out {
		out[i] = Attempt{N: i, SentAt: base.Add(time.Duration(i) * time.Second), OK: true, StatusCode: 200}
	}
	return out
}

func int64Ptr(v int64) *int64        { return &v }
func timePtr(v time.Time) *time.Time { return &v }
