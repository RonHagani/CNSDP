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

func TestBacklogByStatus_PeakAndFinalPerStatus_ExcludesEvidenced(t *testing.T) {
	base := time.Now()
	series := []StatusSample{
		{At: base, Counts: map[string]int64{"admitted": 10, "validated": 2, "evidenced": 100}},
		{At: base.Add(time.Second), Counts: map[string]int64{"admitted": 4, "validated": 9, "evidenced": 150}},
		{At: base.Add(2 * time.Second), Counts: map[string]int64{"admitted": 6, "validated": 5, "evidenced": 200}},
	}

	peak, final := backlogByStatus(series)

	wantPeak := map[string]int64{"admitted": 10, "validated": 9}
	if len(peak) != len(wantPeak) {
		t.Fatalf("peak = %v, want exactly %v (evidenced must be excluded)", peak, wantPeak)
	}
	for status, want := range wantPeak {
		if got := peak[status]; got != want {
			t.Errorf("peak[%q] = %d, want %d", status, got, want)
		}
	}
	if _, ok := peak["evidenced"]; ok {
		t.Error("peak must not contain an 'evidenced' entry")
	}

	wantFinal := map[string]int64{"admitted": 6, "validated": 5}
	if len(final) != len(wantFinal) {
		t.Fatalf("final = %v, want exactly %v (evidenced must be excluded)", final, wantFinal)
	}
	for status, want := range wantFinal {
		if got := final[status]; got != want {
			t.Errorf("final[%q] = %d, want %d", status, got, want)
		}
	}
	if _, ok := final["evidenced"]; ok {
		t.Error("final must not contain an 'evidenced' entry")
	}
}

func TestBacklogByStatus_EmptySeries_ReturnsEmptyMaps(t *testing.T) {
	peak, final := backlogByStatus(nil)
	if len(peak) != 0 {
		t.Errorf("peak = %v, want empty for an empty series", peak)
	}
	if len(final) != 0 {
		t.Errorf("final = %v, want empty for an empty series", final)
	}
}

func TestAC022BacklogDrain_NumbersIncludePerStatusBreakdownAndRawSeries(t *testing.T) {
	base := time.Now()
	series := []StatusSample{
		{At: base, Counts: map[string]int64{"admitted": 3, "normalized": 1, "evidenced": 50}},
		{At: base.Add(time.Second), Counts: map[string]int64{"admitted": 1, "normalized": 2, "evidenced": 60}},
	}
	result := &SustainedRunResult{StatusSeries: series}

	v := ac022BacklogDrain(result)

	if v.Numbers["series"] == nil {
		t.Fatal("expected Numbers[\"series\"] to preserve the raw StatusSeries, got nil")
	}
	gotSeries, ok := v.Numbers["series"].([]StatusSample)
	if !ok || len(gotSeries) != len(series) {
		t.Fatalf("Numbers[\"series\"] = %#v, want the original %d-sample series", v.Numbers["series"], len(series))
	}

	peakByStatus, ok := v.Numbers["peakByStatus"].(map[string]int64)
	if !ok {
		t.Fatalf("Numbers[\"peakByStatus\"] has unexpected type %T", v.Numbers["peakByStatus"])
	}
	if peakByStatus["admitted"] != 3 || peakByStatus["normalized"] != 2 {
		t.Errorf("peakByStatus = %v, want admitted=3, normalized=2", peakByStatus)
	}
	if _, ok := peakByStatus["evidenced"]; ok {
		t.Error("peakByStatus must not contain an 'evidenced' entry")
	}

	finalByStatus, ok := v.Numbers["finalByStatus"].(map[string]int64)
	if !ok {
		t.Fatalf("Numbers[\"finalByStatus\"] has unexpected type %T", v.Numbers["finalByStatus"])
	}
	if finalByStatus["admitted"] != 1 || finalByStatus["normalized"] != 2 {
		t.Errorf("finalByStatus = %v, want admitted=1, normalized=2", finalByStatus)
	}
	if _, ok := finalByStatus["evidenced"]; ok {
		t.Error("finalByStatus must not contain an 'evidenced' entry")
	}

	// Existing aggregate-scalar behavior must be unchanged.
	if v.Numbers["peakBacklog"] != int64(4) {
		t.Errorf("peakBacklog = %v, want 4 (unchanged aggregate behavior)", v.Numbers["peakBacklog"])
	}
	if v.Numbers["finalBacklog"] != int64(3) {
		t.Errorf("finalBacklog = %v, want 3 (unchanged aggregate behavior)", v.Numbers["finalBacklog"])
	}
}

// TestAC022BacklogDrain_NumbersSurviveJSONRoundTrip exercises an actual
// encoding/json Marshal/Unmarshal round trip on the backlog_drain
// Verdict, not just the in-memory Go value: a JSON round trip is the
// only thing report.go's WriteJSON ever actually does with this data
// (json.MarshalIndent, then persisted to disk), so a test that only
// type-asserts the pre-marshal Go map -- as
// TestAC022BacklogDrain_NumbersIncludePerStatusBreakdownAndRawSeries
// above does -- never proves the JSON shape a reader of the report file
// would actually see. encoding/json decodes a map[string]any's nested
// values generically (JSON objects -> map[string]any, JSON numbers ->
// float64, JSON arrays -> []any), so this test checks the decoded shape
// on those terms rather than expecting the original concrete Go types
// (map[string]int64, []StatusSample) back.
func TestAC022BacklogDrain_NumbersSurviveJSONRoundTrip(t *testing.T) {
	base := time.Date(2026, 8, 12, 17, 21, 30, 0, time.UTC)
	series := []StatusSample{
		{At: base, Counts: map[string]int64{"admitted": 10, "validated": 2, "evidenced": 100}},
		{At: base.Add(10 * time.Second), Counts: map[string]int64{"admitted": 4, "validated": 9, "evidenced": 150}},
		{At: base.Add(20 * time.Second), Counts: map[string]int64{"admitted": 6, "validated": 5, "evidenced": 200}},
	}
	result := &SustainedRunResult{StatusSeries: series}

	v := ac022BacklogDrain(result)

	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal(backlog_drain Verdict) failed: %v", err)
	}

	var decoded Verdict
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal into Verdict failed: %v", err)
	}
	if decoded.Clause != "backlog_drain" {
		t.Errorf("decoded.Clause = %q, want %q", decoded.Clause, "backlog_drain")
	}

	// peakByStatus: JSON object -> map[string]any with float64 values.
	peakByStatus, ok := decoded.Numbers["peakByStatus"].(map[string]any)
	if !ok {
		t.Fatalf("decoded Numbers[\"peakByStatus\"] has unexpected type %T, want map[string]any", decoded.Numbers["peakByStatus"])
	}
	if peakByStatus["admitted"] != float64(10) {
		t.Errorf("decoded peakByStatus[\"admitted\"] = %v, want 10", peakByStatus["admitted"])
	}
	if peakByStatus["validated"] != float64(9) {
		t.Errorf("decoded peakByStatus[\"validated\"] = %v, want 9", peakByStatus["validated"])
	}
	if _, ok := peakByStatus["evidenced"]; ok {
		t.Error("decoded peakByStatus must not contain an 'evidenced' entry after the JSON round trip")
	}

	// finalByStatus: same shape, values from the last sample only.
	finalByStatus, ok := decoded.Numbers["finalByStatus"].(map[string]any)
	if !ok {
		t.Fatalf("decoded Numbers[\"finalByStatus\"] has unexpected type %T, want map[string]any", decoded.Numbers["finalByStatus"])
	}
	if finalByStatus["admitted"] != float64(6) {
		t.Errorf("decoded finalByStatus[\"admitted\"] = %v, want 6", finalByStatus["admitted"])
	}
	if finalByStatus["validated"] != float64(5) {
		t.Errorf("decoded finalByStatus[\"validated\"] = %v, want 5", finalByStatus["validated"])
	}
	if _, ok := finalByStatus["evidenced"]; ok {
		t.Error("decoded finalByStatus must not contain an 'evidenced' entry after the JSON round trip")
	}

	// series: JSON array -> []any of JSON objects, one per StatusSample.
	// StatusSample carries no json struct tags (unlike ResourceSample's
	// ac023Verdict precedent, which does), so encoding/json falls back to
	// the exported Go field names verbatim: "At" and "Counts", not
	// lowercase "at"/"counts". Asserting on the field names JSON actually
	// produces here, not the names a tagged struct would use -- this test
	// verifies existing behavior, it does not get to wish the tags into
	// existence.
	gotSeries, ok := decoded.Numbers["series"].([]any)
	if !ok {
		t.Fatalf("decoded Numbers[\"series\"] has unexpected type %T, want []any", decoded.Numbers["series"])
	}
	if len(gotSeries) != len(series) {
		t.Fatalf("decoded series has %d entries, want %d", len(gotSeries), len(series))
	}

	firstSample, ok := gotSeries[0].(map[string]any)
	if !ok {
		t.Fatalf("decoded series[0] has unexpected type %T, want map[string]any", gotSeries[0])
	}
	gotAt, ok := firstSample["At"].(string)
	if !ok {
		t.Fatalf("decoded series[0][\"At\"] has unexpected type %T, want string", firstSample["At"])
	}
	parsedAt, err := time.Parse(time.RFC3339Nano, gotAt)
	if err != nil {
		t.Fatalf("decoded series[0][\"At\"] = %q is not a valid RFC3339 timestamp: %v", gotAt, err)
	}
	if !parsedAt.Equal(series[0].At) {
		t.Errorf("decoded series[0][\"At\"] = %v, want %v", parsedAt, series[0].At)
	}
	firstCounts, ok := firstSample["Counts"].(map[string]any)
	if !ok {
		t.Fatalf("decoded series[0][\"Counts\"] has unexpected type %T, want map[string]any", firstSample["Counts"])
	}
	if firstCounts["admitted"] != float64(10) || firstCounts["validated"] != float64(2) || firstCounts["evidenced"] != float64(100) {
		t.Errorf("decoded series[0][\"Counts\"] = %v, want admitted=10, validated=2, evidenced=100", firstCounts)
	}

	lastSample, ok := gotSeries[len(gotSeries)-1].(map[string]any)
	if !ok {
		t.Fatalf("decoded series[%d] has unexpected type %T, want map[string]any", len(gotSeries)-1, gotSeries[len(gotSeries)-1])
	}
	lastCounts, ok := lastSample["Counts"].(map[string]any)
	if !ok {
		t.Fatalf("decoded series[%d][\"Counts\"] has unexpected type %T, want map[string]any", len(gotSeries)-1, lastSample["Counts"])
	}
	if lastCounts["admitted"] != float64(6) || lastCounts["validated"] != float64(5) || lastCounts["evidenced"] != float64(200) {
		t.Errorf("decoded last series entry counts = %v, want admitted=6, validated=5, evidenced=200", lastCounts)
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
