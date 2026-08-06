// Package refresh holds the rule for whether a presented refresh token is
// still good. Like internal/validate and internal/ratelimit it has no HTTP or
// database dependencies, which is what makes the awkward cases — the grace
// window, the ordering of the checks — testable without either.
package refresh

import "time"

// State is what the database knows about a presented token.
type State struct {
	Found     bool
	ExpiresAt time.Time
	// nil means active. Non-nil records when the token was consumed, which the
	// grace window needs — a boolean could not express "a moment ago".
	RevokedAt *time.Time
	// Whether any token in this family is still alive.
	//
	// This is what separates "revoked because it was rotated" from "revoked
	// because the session ended". Rotation always leaves a live successor;
	// logout and reuse detection revoke the whole family and leave nothing.
	// Without it, a replay within the grace window would resurrect a session
	// the user had explicitly logged out of.
	FamilyActive bool
}

type Decision int

const (
	Accept Decision = iota
	RejectUnknown
	RejectExpired
	// RejectRevoked is a session that is simply over — logged out, or already
	// killed by reuse detection. Nothing more to revoke, and no alarm.
	RejectRevoked
	ReuseDetected
)

func (d Decision) String() string {
	switch d {
	case Accept:
		return "Accept"
	case RejectUnknown:
		return "RejectUnknown"
	case RejectExpired:
		return "RejectExpired"
	case RejectRevoked:
		return "RejectRevoked"
	case ReuseDetected:
		return "ReuseDetected"
	}
	return "Unknown"
}

// Decide reports what to do with a presented refresh token.
//
// The order of the checks is deliberate. Expiry comes before revocation, so a
// token that is both expired and revoked reports RejectExpired: replaying a
// token that can no longer do anything is not worth burning a user's session
// over.
//
// A token revoked within grace is treated as a concurrent retry rather than an
// attack. Two browser tabs share one cookie and refresh independently, so
// without this the second one to arrive would be indistinguishable from a
// stolen token being replayed — and the user would be logged out of both for
// doing nothing.
//
// The grace window applies only while the family still has a live token. A
// revoked token in a dead family is a session that has ended — logged out, or
// already killed by reuse detection — and no grace period should bring it back.
func Decide(state State, now time.Time, grace time.Duration) Decision {
	if !state.Found {
		return RejectUnknown
	}

	// Not Before, so a token expiring exactly now is expired.
	if !now.Before(state.ExpiresAt) {
		return RejectExpired
	}

	if state.RevokedAt != nil {
		if !state.FamilyActive {
			return RejectRevoked
		}
		if now.Sub(*state.RevokedAt) <= grace {
			return Accept
		}
		return ReuseDetected
	}

	return Accept
}
