package igdb

import (
	"context"
	"sync"
	"time"
)

// throttle admits at most a fixed number of events per second, smoothing bursts
// rather than rejecting them.
//
// IGDB allows 4 requests per second and answers 429 beyond that, which would
// surface as a search that mysteriously stops working. A waiter rather than a
// rejecter: a caller arriving too early should be slowed, not failed.
//
// It lives in the client rather than the frontend because a debounce guards one
// caller, and this guards every caller — a script, a curl loop, or the import
// path a later cycle adds.
type throttle struct {
	interval time.Duration

	mu   sync.Mutex
	next time.Time

	// Replaced by tests so pacing is asserted deterministically instead of by
	// sleeping, the same reason internal/ratelimit takes an injectable clock.
	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

func newThrottle(perSecond int) *throttle {
	return &throttle{
		interval: time.Second / time.Duration(perSecond),
		now:      time.Now,
		sleep:    sleepContext,
	}
}

// wait blocks until this caller's slot arrives, or ctx is done.
//
// Each caller claims the next slot under the mutex and releases it before
// sleeping, so callers queue in arrival order without holding the lock across
// a wait.
func (t *throttle) wait(ctx context.Context) error {
	t.mu.Lock()
	now := t.now()
	slot := t.next
	if slot.Before(now) {
		slot = now
	}
	t.next = slot.Add(t.interval)
	t.mu.Unlock()

	delay := slot.Sub(now)
	if delay <= 0 {
		// A free slot is not a free pass. A caller whose context is already
		// done has given up, and spending an IGDB request on it helps nobody.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}

	return t.sleep(ctx, delay)
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
