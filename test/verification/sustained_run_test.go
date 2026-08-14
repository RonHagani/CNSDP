package main

import (
	"encoding/json"
	"net/http"
	"strings"
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

func TestAC022CapacityRejectionDefer_AllAdmitted_Fails(t *testing.T) {
	p := FullParams()
	result := &SustainedRunResult{
		Attempts:      makeAllOKAttempts(10),
		BurstStartIdx: 5,
	}
	v := ac022CapacityRejectionDefer(p, result)
	if v.Status != VerdictFail {
		t.Errorf("Status = %s, want %s when every burst attempt was admitted (no 429s observed)", v.Status, VerdictFail)
	}
}

// smallBurstParams is a self-contained Params fixture (not FullParams,
// whose expectedExcess of 150 needs a real 450-attempt burst window to
// reconcile against) sized so a small, hand-built Attempts slice can
// exercise the reconciliation arithmetic directly: BurstDuration(1s) *
// (BurstOfferedRate(14) - NormalOfferedRate(10)) = an expected excess of
// exactly 4, giving lowerBound=3 and upperBound=4 (int(4*0.85)=3,
// int(4*1.15)=4) -- tight enough that 4 itself exercises the upper
// boundary inclusively.
func smallBurstParams() Params {
	return Params{
		NormalOfferedRate: 10,
		BurstOfferedRate:  14,
		BurstDuration:     1 * time.Second,
	}
}

// mediumBurstParams gives more daylight between lowerBound and upperBound
// than smallBurstParams, so "clearly inside the range" and "clearly above
// the upper bound" can each be tested with a value that is not itself a
// boundary: BurstDuration(2s) * (BurstOfferedRate(20) -
// NormalOfferedRate(10)) = an expected excess of exactly 20, giving
// lowerBound=17 (int(20*0.85)) and upperBound=23 (int(20*1.15)).
func mediumBurstParams() Params {
	return Params{
		NormalOfferedRate: 10,
		BurstOfferedRate:  20,
		BurstDuration:     2 * time.Second,
	}
}

func TestAC022CapacityRejectionDefer_ExpectedExcessRejected_Passes(t *testing.T) {
	p := smallBurstParams() // expectedExcess = 4, upperBound = 4
	attempts := makeAllOKAttempts(10)
	// Burst window = indices 5..9. One admitted, four capacity-rejected --
	// exactly reconciles with the expected excess of 4, and exercises the
	// upper bound inclusively (rejected429 == upperBound must still PASS).
	for i := 6; i <= 9; i++ {
		attempts[i].OK = false
		attempts[i].StatusCode = http.StatusTooManyRequests
	}
	result := &SustainedRunResult{Attempts: attempts, BurstStartIdx: 5}
	v := ac022CapacityRejectionDefer(p, result)
	if v.Status != VerdictPass {
		t.Errorf("Status = %s, want %s: 4 of 5 burst attempts capacity-rejected reconciles with an expected excess of 4 (detail: %s)", v.Status, VerdictPass, v.Detail)
	}
}

func TestAC022CapacityRejectionDefer_WithinRange_Passes(t *testing.T) {
	p := mediumBurstParams() // expectedExcess = 20, accepted range [17, 23]
	attempts := makeAllOKAttempts(30)
	// Burst window = indices 5..29 (25 attempts). Exactly 20 capacity-
	// rejected, comfortably inside the accepted range without sitting on
	// either boundary.
	for i := 5; i < 25; i++ {
		attempts[i].OK = false
		attempts[i].StatusCode = http.StatusTooManyRequests
	}
	result := &SustainedRunResult{Attempts: attempts, BurstStartIdx: 5}
	v := ac022CapacityRejectionDefer(p, result)
	if v.Status != VerdictPass {
		t.Errorf("Status = %s, want %s: 20 of 25 burst attempts capacity-rejected is within the accepted range [17,23] for an expected excess of 20 (detail: %s)", v.Status, VerdictPass, v.Detail)
	}
}

func TestAC022CapacityRejectionDefer_BelowLowerBound_Fails(t *testing.T) {
	p := smallBurstParams() // expectedExcess = 4, lowerBound = 3
	attempts := makeAllOKAttempts(10)
	// Burst window = indices 5..9. Only one capacity-rejected -- well
	// under the expected excess of 4, so this must not report PASS merely
	// because at least one 429 was observed.
	attempts[9].OK = false
	attempts[9].StatusCode = http.StatusTooManyRequests
	result := &SustainedRunResult{Attempts: attempts, BurstStartIdx: 5}
	v := ac022CapacityRejectionDefer(p, result)
	if v.Status != VerdictFail {
		t.Errorf("Status = %s, want %s: 1 of 5 burst attempts capacity-rejected does not reconcile with an expected excess of 4 (detail: %s)", v.Status, VerdictFail, v.Detail)
	}
}

func TestAC022CapacityRejectionDefer_AboveUpperBound_Fails(t *testing.T) {
	p := mediumBurstParams() // expectedExcess = 20, upperBound = 23
	attempts := makeAllOKAttempts(30)
	// Burst window = indices 5..29 (25 attempts). 24 capacity-rejected --
	// above the upper bound of 23, meaning the mechanism is refusing more
	// attempts than the approved envelope's excess actually accounts for
	// (over-enforcing against in-envelope traffic). A lower-bound-only
	// check would have wrongly PASSed this.
	for i := 5; i < 29; i++ {
		attempts[i].OK = false
		attempts[i].StatusCode = http.StatusTooManyRequests
	}
	result := &SustainedRunResult{Attempts: attempts, BurstStartIdx: 5}
	v := ac022CapacityRejectionDefer(p, result)
	if v.Status != VerdictFail {
		t.Errorf("Status = %s, want %s: 24 of 25 burst attempts capacity-rejected exceeds the upper bound of 23 for an expected excess of 20 (detail: %s)", v.Status, VerdictFail, v.Detail)
	}
	rejected429, _ := v.Numbers["rejected429"].(int)
	if rejected429 != 24 {
		t.Errorf("rejected429 = %d, want 24", rejected429)
	}
}

func TestAC022CapacityRejectionDefer_NonCapacityFailure_NotCountedAs429(t *testing.T) {
	p := smallBurstParams()
	attempts := makeAllOKAttempts(10)
	// A non-429 failure (e.g. a genuine platform fault, 503) in the burst
	// window must not be silently counted as a capacity rejection.
	attempts[9].OK = false
	attempts[9].StatusCode = http.StatusServiceUnavailable
	result := &SustainedRunResult{Attempts: attempts, BurstStartIdx: 5}
	v := ac022CapacityRejectionDefer(p, result)
	rejected429, _ := v.Numbers["rejected429"].(int)
	otherNonOK, _ := v.Numbers["otherNonOK"].(int)
	if rejected429 != 0 {
		t.Errorf("rejected429 = %d, want 0 (the only non-OK attempt was a 503, not a 429)", rejected429)
	}
	if otherNonOK != 1 {
		t.Errorf("otherNonOK = %d, want 1", otherNonOK)
	}
	if v.Status != VerdictFail {
		t.Errorf("Status = %s, want %s: a non-429 failure alone must not satisfy the capacity-rejection clause", v.Status, VerdictFail)
	}
}

// TestAC022CapacityRejectionDefer_MixedWithNonCapacityFailures_StillPasses
// proves non-429 responses are excluded from BOTH bounds, not only the
// zero-rejection case above: a batch with a within-range number of real
// 429s plus several unrelated 503s must still PASS on the strength of the
// 429 count alone, and the 503s must never inflate rejected429 nor be
// treated as if they widened the accepted range.
func TestAC022CapacityRejectionDefer_MixedWithNonCapacityFailures_StillPasses(t *testing.T) {
	p := mediumBurstParams() // expectedExcess = 20, accepted range [17, 23]
	attempts := makeAllOKAttempts(30)
	// Burst window = indices 5..29 (25 attempts): 20 real capacity
	// rejections (within range) plus 3 unrelated platform-fault 503s.
	for i := 5; i < 25; i++ {
		attempts[i].OK = false
		attempts[i].StatusCode = http.StatusTooManyRequests
	}
	for i := 25; i < 28; i++ {
		attempts[i].OK = false
		attempts[i].StatusCode = http.StatusServiceUnavailable
	}
	result := &SustainedRunResult{Attempts: attempts, BurstStartIdx: 5}
	v := ac022CapacityRejectionDefer(p, result)
	rejected429, _ := v.Numbers["rejected429"].(int)
	otherNonOK, _ := v.Numbers["otherNonOK"].(int)
	if rejected429 != 20 {
		t.Errorf("rejected429 = %d, want 20 (the 3 unrelated 503s must not be counted)", rejected429)
	}
	if otherNonOK != 3 {
		t.Errorf("otherNonOK = %d, want 3", otherNonOK)
	}
	if v.Status != VerdictPass {
		t.Errorf("Status = %s, want %s: 20 real 429s is within [17,23] regardless of the 3 unrelated 503s (detail: %s)", v.Status, VerdictPass, v.Detail)
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

// admittedRateEnvelopeTestParams keeps ac022AdmittedRateEnvelope's bucket
// width small (1s instead of Full's 10s) so a compact, hand-built Attempts
// slice can cross its violation threshold (10 * 1.15 * 1s = 11.5, so 12+
// admitted attempts landing in the same real 1s bucket is enough) without
// needing hundreds of synthetic attempts.
func admittedRateEnvelopeTestParams() Params {
	return Params{
		NormalOfferedRate: 10,
		RateBucketWidth:   1 * time.Second,
	}
}

// TestAC022AdmittedRateEnvelope_ClusteredActualDispatch_FailsDespiteEvenSchedule
// is the core regression for the SentAt/ActualSentAt fix: a fixture whose
// *scheduled* timestamps are perfectly even (would never violate the
// envelope if bucketed by SentAt, the old behavior) but whose *actual*
// dispatch timestamps all cluster into one real second (simulating a
// catch-up burst after a dispatch stall) must be detected as a violation.
func TestAC022AdmittedRateEnvelope_ClusteredActualDispatch_FailsDespiteEvenSchedule(t *testing.T) {
	p := admittedRateEnvelopeTestParams()
	schedBase := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	realBase := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) // an unrelated clock domain on purpose

	const n = 20
	attempts := make([]Attempt, n)
	for i := 0; i < n; i++ {
		attempts[i] = Attempt{
			N:      i,
			SentAt: schedBase.Add(time.Duration(i) * 100 * time.Millisecond), // perfectly even: 10/s across two 1s buckets
			// All 20 real dispatches land within a 95ms real window --
			// far inside a single 1s bucket -- unlike the even schedule.
			ActualSentAt: realBase.Add(time.Duration(i) * 5 * time.Millisecond),
			OK:           true,
			StatusCode:   200,
		}
		if attempts[i].SentAt.Equal(attempts[i].ActualSentAt) {
			t.Fatalf("attempt %d: SentAt and ActualSentAt unexpectedly equal -- invalid test fixture", i)
		}
	}

	// Confirm the premise: bucketed by SentAt (the pre-fix behavior), this
	// fixture shows exactly 10/s in each of two buckets -- no violation.
	// It must be the clustered ActualSentAt that trips the check below.
	schedBuckets := map[int]int{}
	for _, a := range attempts {
		schedBuckets[int(a.SentAt.Sub(schedBase)/p.RateBucketWidth)]++
	}
	for idx, count := range schedBuckets {
		if count != 10 {
			t.Fatalf("invalid test fixture: schedule bucket %d has %d attempts, want exactly 10", idx, count)
		}
	}

	result := &SustainedRunResult{Attempts: attempts}
	v := ac022AdmittedRateEnvelope(p, result)

	if v.Status != VerdictFail {
		t.Fatalf("Status = %s, want %s: 20 admitted attempts landed in the same real 1s window (clustered ActualSentAt), which must be detected even though SentAt was perfectly even (detail: %s)", v.Status, VerdictFail, v.Detail)
	}
	worstRate, _ := v.Numbers["worstBucketRate"].(float64)
	if worstRate < 19.9 || worstRate > 20.1 {
		t.Errorf("worstBucketRate = %v, want ~20.0 (all 20 admitted attempts fell in one real bucket)", worstRate)
	}
	violations, _ := v.Numbers["violatingBuckets"].(int)
	if violations != 1 {
		t.Errorf("violatingBuckets = %v, want 1", violations)
	}
	if strings.Contains(v.Detail, "no admission-control mechanism exists") {
		t.Errorf("Detail = %q, still contains the stale pre-admission-control causal claim", v.Detail)
	}
}

// TestAC022AdmittedRateEnvelope_EvenActualDispatch_Passes is the control
// case: when real dispatch tracks the schedule evenly (SentAt ==
// ActualSentAt) at exactly the envelope rate, the clause must still PASS --
// proving the fix didn't accidentally tighten the check.
func TestAC022AdmittedRateEnvelope_EvenActualDispatch_Passes(t *testing.T) {
	p := admittedRateEnvelopeTestParams()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	const n = 20
	attempts := make([]Attempt, n)
	for i := 0; i < n; i++ {
		at := base.Add(time.Duration(i) * 100 * time.Millisecond)
		attempts[i] = Attempt{N: i, SentAt: at, ActualSentAt: at, OK: true, StatusCode: 200}
	}

	result := &SustainedRunResult{Attempts: attempts}
	v := ac022AdmittedRateEnvelope(p, result)

	if v.Status != VerdictPass {
		t.Errorf("Status = %s, want %s: real dispatch evenly matched the schedule at exactly the envelope rate (detail: %s)", v.Status, VerdictPass, v.Detail)
	}
	worstRate, _ := v.Numbers["worstBucketRate"].(float64)
	if worstRate < 9.9 || worstRate > 10.1 {
		t.Errorf("worstBucketRate = %v, want ~10.0", worstRate)
	}
}

// TestAC022OfferedAdmittedCounts_UnaffectedByActualDispatchTime guards
// against ac022OfferedAdmittedCounts (and, by construction, the
// BurstStartIdx-based burst/baseline split it relies on) ever accidentally
// starting to depend on ActualSentAt -- that split must stay a test-design
// concept keyed on the scheduled rate, not on real dispatch timing.
func TestAC022OfferedAdmittedCounts_UnaffectedByActualDispatchTime(t *testing.T) {
	p := mediumBurstParams()
	attempts := makeAllOKAttempts(30)
	for i := 5; i < 25; i++ {
		attempts[i].OK = false
		attempts[i].StatusCode = http.StatusTooManyRequests
	}
	// Deliberately scramble ActualSentAt away from SentAt/index order.
	scrambledBase := time.Date(2030, 6, 1, 0, 0, 0, 0, time.UTC)
	for i := range attempts {
		attempts[i].ActualSentAt = scrambledBase.Add(time.Duration(29-i) * time.Millisecond)
	}
	result := &SustainedRunResult{Attempts: attempts, BurstStartIdx: 5}

	v := ac022OfferedAdmittedCounts(p, result)
	offered, _ := v.Numbers["actualOfferedTotal"].(int)
	admitted, _ := v.Numbers["actualAdmittedTotal"].(int)
	burstOffered, _ := v.Numbers["actualBurstOffered"].(int)
	burstAdmitted, _ := v.Numbers["actualBurstAdmitted"].(int)
	if offered != 30 || admitted != 10 {
		t.Errorf("offered/admitted = %d/%d, want 30/10 -- must be unaffected by ActualSentAt", offered, admitted)
	}
	if burstOffered != 25 || burstAdmitted != 5 {
		t.Errorf("burst offered/admitted = %d/%d, want 25/5 -- must be unaffected by ActualSentAt", burstOffered, burstAdmitted)
	}
}
