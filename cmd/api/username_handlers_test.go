package main

import (
	"net/http"
	"strings"
	"testing"
)

func checkUsername(t *testing.T, app *application, name string) usernameAvailability {
	t.Helper()
	w := do(t, app, http.MethodGet, "/usernames/"+name, nil)
	mustStatus(t, w, http.StatusOK)

	var body usernameAvailability
	decodeJSON(t, w, &body)
	return body
}

func TestCheckUsernameFreeNameIsAvailable(t *testing.T) {
	app, _ := newTestApp(t)

	got := checkUsername(t, app, "nobodyhasthis")

	if !got.Available || got.Reason != "" {
		t.Errorf("got %+v, want available with no reason", got)
	}
}

func TestCheckUsernameTakenNameIsNot(t *testing.T) {
	app, tx := newTestApp(t)
	createUser(t, tx, "hype", "public")

	got := checkUsername(t, app, "hype")

	if got.Available || got.Reason != reasonTaken {
		t.Errorf("got %+v, want unavailable/taken", got)
	}
}

func TestCheckUsernameReservedBeatsTaken(t *testing.T) {
	app, tx := newTestApp(t)
	// `admin` is both reserved and a real row in the seeded database. validate
	// runs before the lookup, so reserved must win — otherwise the form would
	// say "taken", implying the name would be free if that account went away.
	createUser(t, tx, "admin", "public")

	got := checkUsername(t, app, "admin")

	if got.Available || got.Reason != reasonReserved {
		t.Errorf("got %+v, want unavailable/reserved", got)
	}
}

func TestCheckUsernameProfaneIsNotAllowed(t *testing.T) {
	app, _ := newTestApp(t)

	got := checkUsername(t, app, "fuck")

	if got.Available || got.Reason != reasonNotAllowed {
		t.Errorf("got %+v, want unavailable/not_allowed", got)
	}
}

func TestCheckUsernameTooShortIsInvalid(t *testing.T) {
	app, _ := newTestApp(t)

	got := checkUsername(t, app, "ab")

	if got.Available || got.Reason != reasonInvalid {
		t.Errorf("got %+v, want unavailable/invalid", got)
	}
}

func TestCheckUsernameOverlongIsInvalidNotAnError(t *testing.T) {
	app, _ := newTestApp(t)

	got := checkUsername(t, app, strings.Repeat("a", 200))

	if got.Available || got.Reason != reasonInvalid {
		t.Errorf("got %+v, want unavailable/invalid", got)
	}
}

func TestCheckUsernameNormalisesBeforeLookup(t *testing.T) {
	app, tx := newTestApp(t)
	createUser(t, tx, "hype", "public")

	got := checkUsername(t, app, "HyPe")

	// The server still normalizes even though the frontend now prevents
	// uppercase: that prevention is UX, not enforcement.
	if got.Available || got.Reason != reasonTaken {
		t.Errorf("got %+v, want unavailable/taken", got)
	}
}

func TestCheckUsernameNeedsNoToken(t *testing.T) {
	app, _ := newTestApp(t)

	// No withAuth: the register form has no token, which is the whole reason
	// this route sits outside AuthRequired.
	w := do(t, app, http.MethodGet, "/usernames/nobodyhasthis", nil)

	mustStatus(t, w, http.StatusOK)
}
