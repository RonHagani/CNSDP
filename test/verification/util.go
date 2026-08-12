package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// sleepUntil blocks until t or ctx cancellation, whichever comes first --
// the absolute-time scheduling primitive the sustained-run phase uses so
// per-attempt scheduling error never accumulates into rate drift over a
// long run (unlike a naive time.Sleep(interval) repeated after each send,
// whose drift would compound by each request's own latency).
func sleepUntil(ctx context.Context, t time.Time) {
	d := time.Until(t)
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// percentile returns the p-th percentile (0..100) of sorted (ascending)
// durations using nearest-rank; sorted must already be sorted ascending
// and non-empty.
func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 1 {
		return sorted[0]
	}
	rank := (p / 100) * float64(len(sorted)-1)
	lo := int(rank)
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := rank - float64(lo)
	return sorted[lo] + time.Duration(frac*float64(sorted[hi]-sorted[lo]))
}

// parseDockerMemory parses one side of `docker stats`'s MemUsage field
// (e.g. "12.3MiB", "512kB", "1.2GiB", "800B") into a byte count. Docker
// reports binary (Ki/Mi/Gi) units for MemUsage in practice; decimal
// (k/M/G, no trailing "i") suffixes are accepted too, defensively, since
// the exact suffix set is a Docker CLI formatting detail, not a documented
// contract.
func parseDockerMemory(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty memory value")
	}

	units := []struct {
		suffix     string
		multiplier float64
	}{
		{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10},
		{"GB", 1e9}, {"MB", 1e6}, {"kB", 1e3},
		{"B", 1},
	}
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			numPart := strings.TrimSpace(strings.TrimSuffix(s, u.suffix))
			n, err := strconv.ParseFloat(numPart, 64)
			if err != nil {
				return 0, fmt.Errorf("parse numeric part of %q: %w", s, err)
			}
			return uint64(n * u.multiplier), nil
		}
	}
	return 0, fmt.Errorf("unrecognized memory unit in %q", s)
}
