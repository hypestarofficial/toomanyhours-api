// Package igdb is a client for the IGDB game database, authenticated through
// Twitch client credentials. Like internal/ratelimit and internal/refresh it
// has no Gin and no GORM: it takes values and returns values.
package igdb

import "errors"

var (
	// ErrNotConfigured means no Twitch credentials were supplied. The API turns
	// this into a 503 rather than refusing to boot, so the project still runs
	// for anyone without a Twitch application of their own.
	ErrNotConfigured = errors.New("igdb: credentials not configured")

	// ErrUpstream means IGDB or Twitch failed. The API turns this into a 502:
	// the failure is upstream, not in the caller's request.
	ErrUpstream = errors.New("igdb: upstream request failed")
)

// Tag is a genre, theme or game mode.
//
// It carries IGDB's own id alongside the name because a later cycle has to map
// these onto local rows, and matching on a string when a stable integer is
// available is how you end up with two entries that differ by a space.
type Tag struct {
	IGDBID int    `json:"igdbId"`
	Name   string `json:"name"`
}

// Game is one search result.
//
// The key is IGDBID, never ID: there is no catalog row behind this yet, and
// both are integers in overlapping ranges, so a mix-up would half-work rather
// than fail loudly.
type Game struct {
	IGDBID int    `json:"igdbId"`
	Title  string `json:"title"`
	// What sort of release this is: main_game, dlc, expansion, bundle,
	// remaster and so on. Named kind rather than category because this
	// product's categories are finished/currently_playing/want_to_play, and
	// two meanings for one word in one codebase is how bugs get written.
	Kind string `json:"kind"`
	// IGDB's id for the game this is an add-on of, or nil. Only DLC and
	// expansions are hidden under a parent in this product, but IGDB sets it
	// on remasters and expanded games too — Skyrim SE points at Skyrim.
	ParentIGDBID *int `json:"parentIgdbId"`
	// Nullable. IGDB has games with no cover art and games with no announced
	// date, both common enough that a client assuming otherwise breaks on a
	// real search.
	Image       *string `json:"image"`
	ReleaseDate *string `json:"releaseDate"`
	// Always non-nil, so JSON carries [] rather than null — the same lesson as
	// GetUserGames, where a nil slice made the frontend crash on an empty list.
	Genres    []Tag `json:"genres"`
	Themes    []Tag `json:"themes"`
	GameModes []Tag `json:"gameModes"`
}
