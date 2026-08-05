package ratelimit

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeClock drives elapsed time without sleeping, so a test for a 15-minute
// window runs instantly and never flakes on a slow machine.
type fakeClock struct{ t time.Time }

func (c *fakeClock) Now() time.Time          { return c.t }
func (c *fakeClock) Advance(d time.Duration) { c.t = c.t.Add(d) }

// newTestLimiter returns a limiter whose clock the test controls.
func newTestLimiter(max int, window time.Duration) (*Limiter, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)}
	l := New(max, window)
	l.now = clock.Now
	return l, clock
}

func TestAllowsUpToTheLimit(t *testing.T) {
	l, _ := newTestLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if ok, _ := l.Check("bob@example.com"); !ok {
			t.Fatalf("attempt %d was refused, expected allowed", i+1)
		}
		l.RecordFailure("bob@example.com")
	}
}

func TestRefusesAfterTheLimit(t *testing.T) {
	l, _ := newTestLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		l.RecordFailure("bob@example.com")
	}

	ok, retryAfter := l.Check("bob@example.com")
	if ok {
		t.Fatal("expected the 4th attempt to be refused")
	}
	if retryAfter != time.Minute {
		t.Fatalf("retryAfter = %v, want %v", retryAfter, time.Minute)
	}
}

func TestRetryAfterCountsDownAsTheWindowDrains(t *testing.T) {
	l, clock := newTestLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		l.RecordFailure("bob@example.com")
	}
	clock.Advance(20 * time.Second)

	ok, retryAfter := l.Check("bob@example.com")
	if ok {
		t.Fatal("expected still refused 20s into a 60s window")
	}
	if retryAfter != 40*time.Second {
		t.Fatalf("retryAfter = %v, want 40s", retryAfter)
	}
}

func TestAllowedAgainOnceTheWindowPasses(t *testing.T) {
	l, clock := newTestLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		l.RecordFailure("bob@example.com")
	}
	clock.Advance(time.Minute + time.Second)

	if ok, _ := l.Check("bob@example.com"); !ok {
		t.Fatal("expected allowed after the window expired")
	}
}

func TestResetImmediatelyUnblocks(t *testing.T) {
	l, _ := newTestLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		l.RecordFailure("bob@example.com")
	}
	l.Reset("bob@example.com")

	if ok, _ := l.Check("bob@example.com"); !ok {
		t.Fatal("expected allowed immediately after Reset")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		l.RecordFailure("bob@example.com")
	}

	if ok, _ := l.Check("alice@example.com"); !ok {
		t.Fatal("blocking one key must not block another")
	}
}

// A blocked key must not have its block extended by further failures, or an
// attacker could hold a real account locked indefinitely by keeping up traffic.
func TestFailuresOnABlockedKeyDoNotExtendTheBlock(t *testing.T) {
	l, clock := newTestLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		l.RecordFailure("bob@example.com")
	}

	// Attacker keeps hammering right up to the moment the window drains.
	for i := 0; i < 10; i++ {
		clock.Advance(5 * time.Second)
		l.RecordFailure("bob@example.com")
	}
	clock.Advance(11 * time.Second) // now 61s after the original three

	if ok, _ := l.Check("bob@example.com"); !ok {
		t.Fatal("block outlasted the window; later failures extended it")
	}
}

// Keys come from attacker-controlled input, so abandoned ones must not
// accumulate forever.
func TestSweepDropsAbandonedKeys(t *testing.T) {
	l, clock := newTestLimiter(3, time.Minute)

	for i := 0; i < sweepThreshold+1; i++ {
		l.RecordFailure(fmt.Sprintf("spray-%d@example.com", i))
	}
	clock.Advance(2 * time.Minute)

	// One more write past the threshold with everything expired triggers a sweep.
	l.RecordFailure("trigger@example.com")

	l.mu.Lock()
	remaining := len(l.failures)
	l.mu.Unlock()

	if remaining != 1 {
		t.Fatalf("after sweep %d keys remain, want 1 (only the trigger)", remaining)
	}
}

// Shared mutable state touched by concurrent HTTP handlers. Run under -race.
func TestConcurrentAccessIsSafe(t *testing.T) {
	l := New(5, time.Minute) // real clock: the fake is not goroutine-safe

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			key := fmt.Sprintf("user-%d@example.com", n%3)
			l.RecordFailure(key)
			l.Check(key)
			if n%7 == 0 {
				l.Reset(key)
			}
		}(i)
	}
	wg.Wait()
}
