package intake

import (
	"sync"
	"testing"
	"time"
)

// newFakeClock returns a manually-advanced clock: now reports the current
// fake time, advance moves it forward. No real sleeps are involved, so
// window-boundary tests are exact and fast.
func newFakeClock(start time.Time) (now func() time.Time, advance func(d time.Duration)) {
	t := start
	return func() time.Time { return t }, func(d time.Duration) { t = t.Add(d) }
}

func TestSlidingWindowLimiter_TenSimultaneous_EleventhRejected(t *testing.T) {
	now, _ := newFakeClock(time.Unix(0, 0))
	l := NewSlidingWindowLimiter(10, time.Second, now)

	for i := 1; i <= 10; i++ {
		if !l.Allow() {
			t.Fatalf("call %d (simultaneous): want allowed, got rejected", i)
		}
	}
	if l.Allow() {
		t.Error("11th simultaneous call: want rejected, got allowed")
	}
}

func TestSlidingWindowLimiter_SpreadWithinWindow_EleventhRejected(t *testing.T) {
	now, advance := newFakeClock(time.Unix(0, 0))
	l := NewSlidingWindowLimiter(10, time.Second, now)

	for i := 1; i <= 10; i++ {
		if !l.Allow() {
			t.Fatalf("call %d at %v: want allowed, got rejected", i, now())
		}
		advance(50 * time.Millisecond)
	}
	// now() is at t0+500ms, still well within the trailing second of the
	// first (t0) admission.
	if l.Allow() {
		t.Error("11th call within the same trailing window: want rejected, got allowed")
	}
}

func TestSlidingWindowLimiter_ExactlyOneWindowAfterOldest_Allows(t *testing.T) {
	start := time.Unix(0, 0)
	now, advance := newFakeClock(start)
	l := NewSlidingWindowLimiter(10, time.Second, now)

	for i := 0; i < 10; i++ {
		if !l.Allow() {
			t.Fatalf("seed call %d: want allowed", i+1)
		}
	}
	advance(start.Add(time.Second).Sub(now())) // advance to exactly oldest+1s

	if !l.Allow() {
		t.Error("at exactly oldest+1s: want allowed (the oldest entry has aged out of the window), got rejected")
	}
}

func TestSlidingWindowLimiter_JustBeforeOneWindowAfterOldest_Rejects(t *testing.T) {
	start := time.Unix(0, 0)
	now, advance := newFakeClock(start)
	l := NewSlidingWindowLimiter(10, time.Second, now)

	for i := 0; i < 10; i++ {
		if !l.Allow() {
			t.Fatalf("seed call %d: want allowed", i+1)
		}
	}
	advance(start.Add(time.Second - time.Nanosecond).Sub(now())) // 1ns short of oldest+1s

	if l.Allow() {
		t.Error("1ns before oldest+1s: want rejected (the oldest entry is still within the window), got allowed")
	}
}

func TestSlidingWindowLimiter_SlidingRefill_OneEntryAtATime(t *testing.T) {
	start := time.Unix(0, 0)
	now, advance := newFakeClock(start)
	l := NewSlidingWindowLimiter(10, time.Second, now)

	// Seed 10 admissions 100ms apart: t=0,100,...,900ms.
	for i := 0; i < 10; i++ {
		if !l.Allow() {
			t.Fatalf("seed call %d at %v: want allowed", i+1, now())
		}
		if i < 9 {
			advance(100 * time.Millisecond)
		}
	}

	// Advance just past the first entry's (t=0) one-second boundary --
	// only that one slot has freed so far.
	advance(101 * time.Millisecond) // now at t=1001ms
	if !l.Allow() {
		t.Fatal("just past the oldest entry's window: want exactly one freed slot, got rejected")
	}
	if l.Allow() {
		t.Error("immediately after consuming the single freed slot: want rejected (the next entry, t=100ms, has not aged out yet), got allowed")
	}
}

func TestSlidingWindowLimiter_LongIdle_NoAccumulatedBurstCredit(t *testing.T) {
	start := time.Unix(0, 0)
	now, advance := newFakeClock(start)
	l := NewSlidingWindowLimiter(10, time.Second, now)

	for i := 0; i < 10; i++ {
		l.Allow()
	}
	advance(1500 * time.Millisecond) // long idle stretch -- every prior entry ages out

	admitted := 0
	for i := 0; i < 20; i++ {
		if l.Allow() {
			admitted++
		}
	}
	if admitted != 10 {
		t.Errorf("admissions immediately after an idle period = %d, want exactly 10 (idle time must not bank extra capacity beyond the envelope)", admitted)
	}
}

// TestSlidingWindowLimiter_ConcurrentAllow_RaceFreeAndBounded exercises
// the mutex under real concurrent callers (go test -race). It uses a real
// clock with a small window so the test stays fast; the assertion is <=
// limit rather than == limit to avoid flakiness if a slow CI host lets the
// burst of goroutines spill past the window boundary -- the property this
// test actually verifies is that concurrent access never lets the bound
// be exceeded, which <= captures without depending on scheduling speed.
func TestSlidingWindowLimiter_ConcurrentAllow_RaceFreeAndBounded(t *testing.T) {
	const limit = 10
	l := NewSlidingWindowLimiter(limit, 200*time.Millisecond, nil)

	const goroutines = 200
	var wg sync.WaitGroup
	var mu sync.Mutex
	admitted := 0

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if l.Allow() {
				mu.Lock()
				admitted++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if admitted > limit {
		t.Errorf("admitted = %d concurrent callers within one window, want <= %d", admitted, limit)
	}
}
