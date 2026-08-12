package main

import "testing"

// TestFullParams_MatchesApprovedAC022Arithmetic pins the exact AC-022
// arithmetic corrected during design review: 900s total = 870s baseline +
// 30s burst (a sub-interval, not appended); burst rate is the window's
// TOTAL offered rate (not additive to the baseline rate); offered counts
// 8700 / 450 / 9150; excess 150. A future edit to FullParams that drifts
// from these numbers breaks this test, not silently changes what AC-022
// verification actually proves.
func TestFullParams_MatchesApprovedAC022Arithmetic(t *testing.T) {
	p := FullParams()

	if got, want := p.TotalRunDuration().Seconds(), 900.0; got != want {
		t.Errorf("TotalRunDuration = %v, want %v", got, want)
	}
	if got, want := p.BaselineDuration.Seconds(), 870.0; got != want {
		t.Errorf("BaselineDuration = %v, want %v", got, want)
	}
	if got, want := p.BurstDuration.Seconds(), 30.0; got != want {
		t.Errorf("BurstDuration = %v, want %v", got, want)
	}
	if got, want := p.NormalOfferedRate, 10.0; got != want {
		t.Errorf("NormalOfferedRate = %v, want %v", got, want)
	}
	if got, want := p.BurstOfferedRate, 15.0; got != want {
		t.Errorf("BurstOfferedRate = %v, want %v (TOTAL offered rate during the window, not additive)", got, want)
	}
	if got, want := p.TotalOfferedNormal(), 8700; got != want {
		t.Errorf("TotalOfferedNormal() = %d, want %d", got, want)
	}
	if got, want := p.TotalOfferedBurst(), 450; got != want {
		t.Errorf("TotalOfferedBurst() = %d, want %d", got, want)
	}
	if got, want := p.TotalOffered(), 9150; got != want {
		t.Errorf("TotalOffered() = %d, want %d", got, want)
	}
	if got, want := p.ExcessBurstAttempts(), 150; got != want {
		t.Errorf("ExcessBurstAttempts() = %d, want %d", got, want)
	}
}

func TestSmokeParams_NeverEqualsFullParams(t *testing.T) {
	s, f := SmokeParams(), FullParams()
	if s.TotalOffered() >= f.TotalOffered() {
		t.Errorf("smoke workload (%d) should be far smaller than full (%d)", s.TotalOffered(), f.TotalOffered())
	}
	if s.Profile != ProfileSmoke || f.Profile != ProfileFull {
		t.Errorf("profile tags: smoke=%s full=%s", s.Profile, f.Profile)
	}
}

func TestParamsFor_UnknownProfile_ReturnsEnvironmentError(t *testing.T) {
	_, err := ParamsFor(Profile("bogus"))
	if err == nil {
		t.Fatal("expected an error for an unknown profile")
	}
	var envErr *EnvironmentError
	if !asEnvironmentError(err, &envErr) {
		t.Errorf("expected *EnvironmentError, got %T: %v", err, err)
	}
}

func asEnvironmentError(err error, target **EnvironmentError) bool {
	e, ok := err.(*EnvironmentError)
	if ok {
		*target = e
	}
	return ok
}
