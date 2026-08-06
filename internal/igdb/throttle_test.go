package igdb

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A frozen clock plus a recording sleep means the pacing is asserted exactly,
// with no real time passing. Sleeping in a test buys nothing but flakiness.
func newTestThrottle(perSecond int) (*throttle, *[]time.Duration) {
	slept := []time.Duration{}
	frozen := time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

	t := newThrottle(perSecond)
	t.now = func() time.Time { return frozen }
	t.sleep = func(ctx context.Context, d time.Duration) error {
		slept = append(slept, d)
		return nil
	}
	return t, &slept
}

func TestThrottlePacesCallers(t *testing.T) {
	th, slept := newTestThrottle(4)
	ctx := context.Background()

	for i := 0; i < 4; i++ {
		if err := th.wait(ctx); err != nil {
			t.Fatalf("wait %d: %v", i, err)
		}
	}

	// The first caller has a slot immediately, so it never sleeps. Each later
	// caller waits one more interval than the last: 4/sec is one every 250ms.
	want := []time.Duration{250 * time.Millisecond, 500 * time.Millisecond, 750 * time.Millisecond}
	if len(*slept) != len(want) {
		t.Fatalf("slept %v, want %v", *slept, want)
	}
	for i, d := range want {
		if (*slept)[i] != d {
			t.Fatalf("sleep %d = %v, want %v (all: %v)", i, (*slept)[i], d, *slept)
		}
	}
}

// The global 3-second timeout middleware means a request can be abandoned while
// a caller is queued. Sleeping through that would hold the request past its
// deadline and look like an unexplained timeout.
func TestThrottleHonoursCancelledContextWhileWaiting(t *testing.T) {
	th, _ := newTestThrottle(4)
	th.sleep = func(ctx context.Context, d time.Duration) error { return ctx.Err() }

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_ = th.wait(context.Background()) // consume the free slot
	if err := th.wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait on cancelled context = %v, want context.Canceled", err)
	}
}

// A free slot must not be a free pass: a caller that has already given up
// should be told so rather than proceeding to spend a request on IGDB.
func TestThrottleHonoursCancelledContextWhenSlotIsFree(t *testing.T) {
	th, _ := newTestThrottle(4)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := th.wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("first wait on cancelled context = %v, want context.Canceled", err)
	}
}
