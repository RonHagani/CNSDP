package main

import (
	"testing"
	"time"
)

func TestPercentile(t *testing.T) {
	sorted := []time.Duration{
		1 * time.Second, 2 * time.Second, 3 * time.Second, 4 * time.Second, 5 * time.Second,
		6 * time.Second, 7 * time.Second, 8 * time.Second, 9 * time.Second, 10 * time.Second,
	}
	if got, want := percentile(sorted, 0), 1*time.Second; got != want {
		t.Errorf("p0 = %v, want %v", got, want)
	}
	if got, want := percentile(sorted, 100), 10*time.Second; got != want {
		t.Errorf("p100 = %v, want %v", got, want)
	}
	if got, want := percentile(sorted, 50), 5500*time.Millisecond; got != want {
		t.Errorf("p50 = %v, want %v", got, want)
	}
	single := []time.Duration{42 * time.Second}
	if got := percentile(single, 95); got != 42*time.Second {
		t.Errorf("single-element p95 = %v, want 42s", got)
	}
}

func TestParseDockerMemory(t *testing.T) {
	// Computed via runtime float64 vars (not a constant expression) so it
	// truncates exactly like parseDockerMemory's own uint64(n*multiplier)
	// does -- 12.3 MiB is not an integer number of bytes.
	var n12_3, mib float64 = 12.3, 1 << 20
	want12_3MiB := uint64(n12_3 * mib)

	cases := []struct {
		in   string
		want uint64
	}{
		{"800B", 800},
		{"12.3MiB", want12_3MiB},
		{"1GiB", 1 << 30},
		{"512kB", 512000},
		{"1.5GB", 1500000000},
	}
	for _, c := range cases {
		got, err := parseDockerMemory(c.in)
		if err != nil {
			t.Errorf("parseDockerMemory(%q) error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseDockerMemory(%q) = %d, want %d", c.in, got, c.want)
		}
	}

	if _, err := parseDockerMemory("bogus"); err == nil {
		t.Error("expected an error for an unrecognized unit")
	}
	if _, err := parseDockerMemory(""); err == nil {
		t.Error("expected an error for an empty string")
	}
}
