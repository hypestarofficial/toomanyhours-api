package refresh

import (
	"testing"
	"time"
)

var now = time.Date(2026, 8, 6, 12, 0, 0, 0, time.UTC)

func ago(d time.Duration) *time.Time {
	t := now.Add(-d)
	return &t
}

func TestDecide(t *testing.T) {
	const grace = 10 * time.Second

	tests := []struct {
		name  string
		state State
		want  Decision
	}{
		{
			name:  "no row for this jti",
			state: State{Found: false},
			want:  RejectUnknown,
		},
		{
			name:  "active and unexpired",
			state: State{Found: true, ExpiresAt: now.Add(time.Hour)},
			want:  Accept,
		},
		{
			name:  "expired",
			state: State{Found: true, ExpiresAt: now.Add(-time.Second)},
			want:  RejectExpired,
		},
		{
			name:  "expiring exactly now counts as expired",
			state: State{Found: true, ExpiresAt: now},
			want:  RejectExpired,
		},
		{
			name:  "revoked long ago is a replay",
			state: State{Found: true, ExpiresAt: now.Add(time.Hour), RevokedAt: ago(time.Hour), FamilyActive: true},
			want:  ReuseDetected,
		},
		{
			name:  "revoked a moment ago is a concurrent retry",
			state: State{Found: true, ExpiresAt: now.Add(time.Hour), RevokedAt: ago(2 * time.Second), FamilyActive: true},
			want:  Accept,
		},
		{
			name:  "revoked exactly at the grace boundary is still benign",
			state: State{Found: true, ExpiresAt: now.Add(time.Hour), RevokedAt: ago(grace), FamilyActive: true},
			want:  Accept,
		},
		{
			name:  "one nanosecond past the boundary is a replay",
			state: State{Found: true, ExpiresAt: now.Add(time.Hour), RevokedAt: ago(grace + time.Nanosecond), FamilyActive: true},
			want:  ReuseDetected,
		},
		{
			// Replaying a token that can no longer do anything is not worth
			// burning a user's session over, so expiry wins.
			name:  "expired and revoked reports expired, not reuse",
			state: State{Found: true, ExpiresAt: now.Add(-time.Hour), RevokedAt: ago(time.Hour), FamilyActive: true},
			want:  RejectExpired,
		},
		{
			// The bug the manual verification caught: after logout the whole
			// family is revoked, so a replay inside the grace window must not
			// be mistaken for a concurrent tab and resurrect the session.
			name:  "revoked moments ago but the family is dead is a logged-out session",
			state: State{Found: true, ExpiresAt: now.Add(time.Hour), RevokedAt: ago(time.Second)},
			want:  RejectRevoked,
		},
		{
			name:  "revoked long ago with a dead family is still just over",
			state: State{Found: true, ExpiresAt: now.Add(time.Hour), RevokedAt: ago(time.Hour)},
			want:  RejectRevoked,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := Decide(tc.state, now, grace); got != tc.want {
				t.Fatalf("Decide() = %v, want %v", got, tc.want)
			}
		})
	}
}

// With no grace window every replay is an attack, which is what the manual
// verification in the plan relies on to test reuse detection without waiting.
func TestZeroGraceTreatsAnyReplayAsReuse(t *testing.T) {
	state := State{Found: true, ExpiresAt: now.Add(time.Hour), RevokedAt: ago(time.Nanosecond), FamilyActive: true}

	if got := Decide(state, now, 0); got != ReuseDetected {
		t.Fatalf("Decide() = %v, want ReuseDetected", got)
	}
}
