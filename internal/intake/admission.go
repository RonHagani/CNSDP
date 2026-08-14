// Admission-control gate for the telemetry intake module (NFR-003,
// NFR-004, NFR-013; AC-022): a process-global, in-memory, exact trailing
// one-second sliding-window limiter deciding whether one more submission
// may be admitted right now.
//
// Deliberately not:
//   - a fixed wall-clock-second window, which allows a boundary-doubling
//     burst (up to ~2x the envelope for an instant spanning two windows);
//   - a token bucket, which banks unused capacity into an undocumented
//     burst allowance whenever the offered load is briefly below the
//     envelope;
//   - a minimum-inter-admission-interval (depth-1 leaky-bucket) gate,
//     which would impose uniform spacing between admissions that NFR-003
//     ("10 submissions per second sustained"), NFR-004, and AC-022 ("the
//     admitted rate remains within the supported envelope throughout")
//     never state -- ten submissions admitted together in the same
//     instant is still only ten within that second, and the approved
//     text does not forbid it.
//
// The exact trailing window is the only one of these whose observable
// behavior is precisely "at most 10 admitted in any trailing second," no
// more and no less: it stores only the timestamps of the last (at most)
// AdmissionLimit admitted submissions and evaluates the bound against the
// continuous trailing window at the moment of each admission attempt,
// rather than against a coarser or looser approximation of it.
package intake

import (
	"sync"
	"time"
)

// AdmissionLimit and AdmissionWindow are the approved v0.1 capacity
// envelope (NFR-003): at most 10 admitted submissions per trailing
// one-second window, documented here as the single source cmd/platform
// wires into the production intake Handler.
const (
	AdmissionLimit  = 10
	AdmissionWindow = time.Second
)

// Limiter is satisfied by *SlidingWindowLimiter. Handler.Limiter is a
// Limiter, not a concrete *SlidingWindowLimiter, purely so tests can
// substitute a fake-clock-driven instance without changing Handler's
// field type.
type Limiter interface {
	// Allow reports whether one more submission may be admitted right
	// now, recording the admission if so.
	Allow() bool
}

// SlidingWindowLimiter is the concrete Limiter: at most limit admissions
// are allowed within any trailing window-duration interval. Safe for
// concurrent use -- internal/intake's HTTP handler runs on a goroutine
// per request, and every request shares one process-global instance.
type SlidingWindowLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	// times holds the timestamps of the currently-in-window admissions,
	// oldest first. It never grows past limit entries: a new timestamp is
	// appended only after confirming len(times) < limit.
	times []time.Time
	// now is the clock source, overridable so tests can drive window
	// boundaries deterministically without real sleeps. Defaults to
	// time.Now (see NewSlidingWindowLimiter).
	now func() time.Time
}

// NewSlidingWindowLimiter constructs a limiter admitting at most limit
// submissions per trailing window. A nil now defaults to time.Now; tests
// pass an injected clock instead.
func NewSlidingWindowLimiter(limit int, window time.Duration, now func() time.Time) *SlidingWindowLimiter {
	if now == nil {
		now = time.Now
	}
	return &SlidingWindowLimiter{limit: limit, window: window, now: now}
}

// Allow evaluates and updates the window in one atomic step:
//  1. drop every stored timestamp that is no longer within the trailing
//     window (timestamp <= now-window has aged out -- strict "after
//     cutoff" membership, so an entry exactly window old has already
//     left the window and its slot is immediately available again, not
//     one instant later);
//  2. if fewer than limit timestamps remain, record now and admit;
//  3. otherwise reject without recording anything.
//
// This makes the bound exact and continuous, not merely true on average:
// at every possible instant, at most limit admissions fall within the
// preceding window-duration interval.
func (l *SlidingWindowLimiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	cutoff := now.Add(-l.window)

	i := 0
	for i < len(l.times) && !l.times[i].After(cutoff) {
		i++
	}
	l.times = l.times[i:]

	if len(l.times) >= l.limit {
		return false
	}
	l.times = append(l.times, now)
	return true
}
