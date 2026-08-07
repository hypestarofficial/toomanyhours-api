package main

import (
	"net/http"
	"testing"
	"time"

	"toomanyhours-api/internal/models"
)

const refreshCookieName = "__Host-refresh_token"

func TestRefreshRotatesTheToken(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	pairs := startSession(t, app, user)
	oldJTI := jtiOf(t, app, pairs.RefreshToken)

	w := do(t, app, http.MethodGet, "/refresh-token", nil, withCookie(refreshCookieName, pairs.RefreshToken))

	mustStatus(t, w, http.StatusOK)

	var body TokenPairs
	decodeJSON(t, w, &body)
	if body.Token == "" {
		t.Error("no access token in the response")
	}

	old := refreshRow(t, tx, oldJTI)
	if old.RevokedAt == nil {
		t.Error("the presented token was not revoked — refreshing must be single use")
	}

	// The successor continues the same family, which is what makes logout
	// able to end one device's session and leave the others alone.
	var successors int64
	tx.Model(&models.RefreshToken{}).
		Where("family_id = ? AND jti <> ? AND revoked_at IS NULL", old.FamilyID, oldJTI).
		Count(&successors)
	if successors != 1 {
		t.Errorf("live successors in the family = %d, want 1", successors)
	}
}

func TestReuseOutsideGraceRevokesTheFamily(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	pairs := startSession(t, app, user)
	oldJTI := jtiOf(t, app, pairs.RefreshToken)

	// Rotate once, so the presented token is consumed and a successor exists.
	do(t, app, http.MethodGet, "/refresh-token", nil, withCookie(refreshCookieName, pairs.RefreshToken))

	// Push the revocation out of the grace window. Two parties holding one
	// consumed token means a copy leaked.
	familyID := refreshRow(t, tx, oldJTI).FamilyID
	past := time.Now().Add(-time.Hour)
	if err := tx.Model(&models.RefreshToken{}).Where("jti = ?", oldJTI).Update("revoked_at", past).Error; err != nil {
		t.Fatalf("age the revocation: %v", err)
	}

	w := do(t, app, http.MethodGet, "/refresh-token", nil, withCookie(refreshCookieName, pairs.RefreshToken))

	mustStatus(t, w, http.StatusUnauthorized)

	var live int64
	tx.Model(&models.RefreshToken{}).Where("family_id = ? AND revoked_at IS NULL", familyID).Count(&live)
	if live != 0 {
		t.Errorf("live tokens in the family = %d, want 0 — reuse must end the whole chain", live)
	}
}

func TestReuseInsideGraceSucceeds(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	pairs := startSession(t, app, user)

	do(t, app, http.MethodGet, "/refresh-token", nil, withCookie(refreshCookieName, pairs.RefreshToken))

	// Immediately, so revoked_at is well inside the 10s window. Two browser
	// tabs share one cookie and refresh independently; session.ts's
	// single-flight guard is per JavaScript context and cannot help.
	w := do(t, app, http.MethodGet, "/refresh-token", nil, withCookie(refreshCookieName, pairs.RefreshToken))

	mustStatus(t, w, http.StatusOK)
}

func TestReuseInsideGraceAfterLogoutFails(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	pairs := startSession(t, app, user)

	rotated := do(t, app, http.MethodGet, "/refresh-token", nil, withCookie(refreshCookieName, pairs.RefreshToken))
	mustStatus(t, rotated, http.StatusOK)
	var fresh TokenPairs
	decodeJSON(t, rotated, &fresh)

	// Logout leaves no successor, which is what tells "revoked by rotation"
	// apart from "revoked by logout". Without that distinction a replay inside
	// the window resurrects a session the user explicitly ended.
	do(t, app, http.MethodGet, "/logout", nil, withCookie(refreshCookieName, fresh.RefreshToken))

	w := do(t, app, http.MethodGet, "/refresh-token", nil, withCookie(refreshCookieName, pairs.RefreshToken))

	mustStatus(t, w, http.StatusUnauthorized)
}

func TestExpiredTokenDoesNotRevokeTheFamily(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	pairs := startSession(t, app, user)
	jti := jtiOf(t, app, pairs.RefreshToken)

	past := time.Now().Add(-time.Hour)
	if err := tx.Model(&models.RefreshToken{}).Where("jti = ?", jti).Update("expires_at", past).Error; err != nil {
		t.Fatalf("expire the token: %v", err)
	}

	w := do(t, app, http.MethodGet, "/refresh-token", nil, withCookie(refreshCookieName, pairs.RefreshToken))

	mustStatus(t, w, http.StatusUnauthorized)

	// Expiry is not evidence of theft. Burning the session for it would log
	// people out for going away for a day.
	row := refreshRow(t, tx, jti)
	if row.RevokedAt != nil {
		t.Error("an expired token revoked its own row — expiry is not reuse")
	}
}

func TestAccessTokenIsNotAcceptedAsRefreshCookie(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	pairs := startSession(t, app, user)

	// Access tokens carry no jti and /refresh-token requires one. The handler
	// previously read claims.Subject without checking the token type, so an
	// access token could be exchanged for a fresh pair.
	//
	// This asserts the property, not one mechanism. Verified by removing the
	// `claims.ID == ""` guard, which does NOT make this fail: an empty jti
	// then reaches GetRefreshToken, matches no row, and Decide answers
	// RejectUnknown. The property is defended twice over, and the guard is
	// the cheaper of the two — it does not touch the database.
	w := do(t, app, http.MethodGet, "/refresh-token", nil, withCookie(refreshCookieName, pairs.Token))

	mustStatus(t, w, http.StatusUnauthorized)
}

func TestLogoutWithNoUsableCookieStillReturns200(t *testing.T) {
	app, _ := newTestApp(t)

	// Logout must not be able to fail, and telling an unauthenticated caller
	// their token was invalid is information this endpoint has no business
	// handing out.
	noCookie := do(t, app, http.MethodGet, "/logout", nil)
	mustStatus(t, noCookie, http.StatusOK)

	garbage := do(t, app, http.MethodGet, "/logout", nil, withCookie(refreshCookieName, "not.a.jwt"))
	mustStatus(t, garbage, http.StatusOK)
}

func TestLogoutRevokesOnlyTheCallingFamily(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")

	phone := startSession(t, app, user)
	laptop := startSession(t, app, user)

	do(t, app, http.MethodGet, "/logout", nil, withCookie(refreshCookieName, phone.RefreshToken))

	phoneRow := refreshRow(t, tx, jtiOf(t, app, phone.RefreshToken))
	if phoneRow.RevokedAt == nil {
		t.Error("the calling family was not revoked")
	}

	// Logging out on one device must leave the others alone — that is the
	// whole reason a family id exists.
	w := do(t, app, http.MethodGet, "/refresh-token", nil, withCookie(refreshCookieName, laptop.RefreshToken))
	mustStatus(t, w, http.StatusOK)
}
