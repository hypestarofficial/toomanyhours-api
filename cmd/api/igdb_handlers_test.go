package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestGetDLCsRequiresIGDBConfigured(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")

	// The test app's IGDB client points at an unroutable address; nil is the
	// "no credentials on this deployment" case, which is a 503 rather than a
	// 502 because nothing upstream was even attempted.
	app.IGDB = nil

	w := do(t, app, http.MethodGet, "/games/1942/dlcs", nil, withAuth(accessToken(t, app, user)))

	mustStatus(t, w, http.StatusServiceUnavailable)
}

func TestGetDLCsRejectsANonIntegerID(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")

	w := do(t, app, http.MethodGet, "/games/banana/dlcs", nil, withAuth(accessToken(t, app, user)))

	mustStatus(t, w, http.StatusBadRequest)
}

// /games/search is a static segment and /games/:igdbId/dlcs a parameterised
// one. CLAUDE.md records that this adjacency was worth re-checking whenever a
// /games/:something route returns, and one has.
//
// It was checked, and the honest result is that there is nothing to catch:
// Gin gives a static segment priority over a parameter, and this test still
// passes when the route is shortened to /games/:igdbId *and* when it is
// registered ahead of /games/search. Both were tried. So this is a cheap
// regression guard against a future router change, not proof of a live hazard
// — and it should not be cited as evidence that the ordering is delicate.
func TestSearchRouteStillWinsOverTheDLCWildcard(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")

	w := do(t, app, http.MethodGet, "/games/search", nil, withAuth(accessToken(t, app, user)))

	// Both handlers answer 400 here, so the status alone proves nothing — the
	// message is what says which one ran. "search" is not an integer, so the
	// DLC handler would reject the id instead.
	mustStatus(t, w, http.StatusBadRequest)
	if body := w.Body.String(); !strings.Contains(body, "q must be") {
		t.Errorf("body = %s, want the search handler's message — the DLC route shadowed /games/search", body)
	}
}

func TestGetDLCsRequiresAToken(t *testing.T) {
	app, _ := newTestApp(t)

	w := do(t, app, http.MethodGet, "/games/1942/dlcs", nil)

	mustStatus(t, w, http.StatusUnauthorized)
}
