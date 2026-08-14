package main

import (
	"net/http"
	"testing"
	"toomanyhours-api/internal/igdb"
	"toomanyhours-api/internal/repository/dbrepo"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type healthBody struct {
	Status string `json:"status"`
	Checks struct {
		Database string `json:"database"`
		IGDB     string `json:"igdb"`
	} `json:"checks"`
	// Absent on purpose — see TestHealthPublishesNoVersion.
	Version string `json:"version"`
}

// deadRepo is a repository pointed at a port nothing listens on, so a ping
// fails immediately rather than after a connect timeout.
//
// DisableAutomaticPing is what makes it constructible: gorm.Open pings on open
// by default and would refuse to return a handle at all. A real pool that
// cannot reach a real address is worth more here than a fake that returns a
// canned error — the thing under test is that the handler notices.
func deadRepo(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(
		postgres.Open("host=127.0.0.1 port=1 user=nobody password=nobody dbname=nothing sslmode=disable connect_timeout=1"),
		&gorm.Config{DisableAutomaticPing: true},
	)
	if err != nil {
		t.Fatalf("open dead handle: %v", err)
	}
	return db
}

func TestHealthIsOKWhenTheDatabaseAnswers(t *testing.T) {
	app, _ := newTestApp(t)

	w := do(t, app, http.MethodGet, "/", nil)
	mustStatus(t, w, http.StatusOK)

	var body healthBody
	decodeJSON(t, w, &body)

	if body.Status != "ok" {
		t.Errorf("status = %q, want ok", body.Status)
	}
	if body.Checks.Database != "ok" {
		t.Errorf("checks.database = %q, want ok", body.Checks.Database)
	}
}

// The whole point of the change. The old handler returned a string literal, so
// it answered 200 whether or not Postgres was reachable — a constant shaped
// like a health check, which reports a dead machine as a live one to anything
// that decides on it.
func TestHealthIs503WhenTheDatabaseIsUnreachable(t *testing.T) {
	app, _ := newTestApp(t)
	app.DB = &dbrepo.PostgresDBRepo{GormDB: deadRepo(t)}

	w := do(t, app, http.MethodGet, "/", nil)
	mustStatus(t, w, http.StatusServiceUnavailable)

	var body healthBody
	decodeJSON(t, w, &body)

	if body.Status == "ok" {
		t.Error("status is ok while the database is unreachable")
	}
	if body.Checks.Database == "ok" {
		t.Error("checks.database is ok while the database is unreachable")
	}
}

// Never a failure: this deployment is designed to boot without Twitch
// credentials, so missing ones are a fact to report rather than an outage.
func TestHealthReportsIGDBWithoutFailing(t *testing.T) {
	app, _ := newTestApp(t)

	app.IGDB = nil
	w := do(t, app, http.MethodGet, "/", nil)
	mustStatus(t, w, http.StatusOK)

	var absent healthBody
	decodeJSON(t, w, &absent)
	if absent.Checks.IGDB != "not_configured" {
		t.Errorf("checks.igdb = %q, want not_configured", absent.Checks.IGDB)
	}

	app.IGDB = igdb.New(igdb.Config{ClientID: "id", ClientSecret: "secret"})
	w = do(t, app, http.MethodGet, "/", nil)
	mustStatus(t, w, http.StatusOK)

	var present healthBody
	decodeJSON(t, w, &present)
	if present.Checks.IGDB != "configured" {
		t.Errorf("checks.igdb = %q, want configured", present.Checks.IGDB)
	}
}

// The constant had not moved in months and contradicted a decision on record:
// the API has no version of its own. A stale version is worse than none,
// because eventually somebody believes it. This route is also unauthenticated
// and world-readable, so every number on it is free fingerprinting.
func TestHealthPublishesNoVersion(t *testing.T) {
	app, _ := newTestApp(t)

	w := do(t, app, http.MethodGet, "/", nil)
	mustStatus(t, w, http.StatusOK)

	var body healthBody
	decodeJSON(t, w, &body)

	if body.Version != "" {
		t.Errorf("version = %q, want the field gone entirely", body.Version)
	}
}
