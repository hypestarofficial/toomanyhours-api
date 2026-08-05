// Package ratelimit counts recent failures per key over a sliding window. Like
// internal/validate it is free of HTTP and database dependencies, which is what
// makes it testable in isolation: it takes values and returns values.
package ratelimit

import (
	"sync"
	"time"
)

// sweepThreshold is how many live keys must exist before a sweep is worth its
// cost. Below it, lazy pruning on access already keeps the map small.
const sweepThreshold = 1024

// Limiter allows at most max failures per key within window. The zero value is
// not usable; construct with New. Safe for concurrent use.
type Limiter struct {
	max    int
	window time.Duration

	mu        sync.Mutex
	failures  map[string][]time.Time
	lastSweep time.Time

	// now is a field rather than a direct time.Now call so tests can drive
	// elapsed time instead of sleeping. Only tests in this package replace it,
	// and only before the limiter is shared with any other goroutine.
	now func() time.Time
}

// New returns a Limiter allowing max failures per key within window.
func New(max int, window time.Duration) *Limiter {
	return &Limiter{
		max:      max,
		window:   window,
		failures: make(map[string][]time.Time),
		now:      time.Now,
	}
}

// Check reports whether key may make another attempt. When ok is false,
// retryAfter is how long until the oldest recorded failure leaves the window.
func (l *Limiter) Check(key string) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	times := l.pruneKey(key, now)
	if len(times) < l.max {
		return true, 0
	}
	return false, l.window - now.Sub(times[0])
}

// RecordFailure notes a failed attempt for key.
//
// A key already at the limit is deliberately left alone. If further failures
// appended, an attacker who knows someone's email could hold that account
// blocked indefinitely by keeping up a trickle of guesses. Leaving it means the
// block always expires one window after the failures that caused it. It also
// bounds each key's slice at max entries.
func (l *Limiter) RecordFailure(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.maybeSweep(now)

	times := l.pruneKey(key, now)
	if len(times) >= l.max {
		return
	}
	l.failures[key] = append(times, now)
}

// Reset clears a key's history. Called after a successful login, so a user who
// fumbled their password three times is not punished once they get in.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

// pruneKey drops expired timestamps for one key and returns what survives,
// deleting the key entirely when nothing does. Caller must hold l.mu.
func (l *Limiter) pruneKey(key string, now time.Time) []time.Time {
	times, seen := l.failures[key]
	if !seen {
		return nil
	}

	kept := prune(times, now.Add(-l.window))
	if len(kept) == 0 {
		delete(l.failures, key)
		return nil
	}
	l.failures[key] = kept
	return kept
}

// maybeSweep drops keys whose failures have all expired. Keys are
// attacker-supplied — any string sent as an email address becomes one — so
// without this the map grows without limit under a spraying attack. Rate-limited
// to once per window so a busy server does not rescan constantly. Caller must
// hold l.mu.
func (l *Limiter) maybeSweep(now time.Time) {
	if len(l.failures) < sweepThreshold || now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now

	cutoff := now.Add(-l.window)
	for key, times := range l.failures {
		if kept := prune(times, cutoff); len(kept) == 0 {
			delete(l.failures, key)
		} else {
			l.failures[key] = kept
		}
	}
}

// prune returns the timestamps strictly after cutoff. Entries are appended in
// time order, so the survivors are always a suffix and one scan is enough. An
// entry exactly at the cutoff is exactly one window old, and so has expired.
func prune(times []time.Time, cutoff time.Time) []time.Time {
	for i, t := range times {
		if t.After(cutoff) {
			return times[i:]
		}
	}
	return nil
}
