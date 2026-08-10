package models

import (
	"time"

	"toomanyhours-api/internal/igdb"
)

// Tag is a genre, theme or game mode — three lists IGDB attaches to a game that
// behave identically, so they share one table keyed by (facet, igdb_id).
//
// Facet and IGDBID are json:"-": they are how rows are identified and deduped
// on import, not something a client needs. What ships is {id, name}.
type Tag struct {
	ID        int       `json:"id" gorm:"primaryKey"`
	Facet     string    `json:"-"`
	IGDBID    int       `json:"-" gorm:"column:igdb_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// Game is a catalog row.
//
// ID is a surrogate key that means nothing outside this database; IGDBID is
// IGDB's identifier. Keeping them separate is the whole point of migration
// 000006 — the previous scheme used Steam's ids as the primary key, and Steam
// and IGDB number games in the same range.
type Game struct {
	ID     int    `json:"id" gorm:"primaryKey"`
	IGDBID int    `json:"igdbId" gorm:"column:igdb_id"`
	Title  string `json:"title"`
	Image  string `json:"image"`
	// What sort of release this is: main_game, dlc, expansion, bundle,
	// remaster... Never "category", which in this product means
	// finished/currently_playing/want_to_play.
	Kind string `json:"kind"`
	// IGDB's id for the game this one is an add-on of, or nil. Nullable and
	// not a foreign key: the parent is very often not in the catalog. A
	// pointer because 0 would be an id, and "no parent" is not game zero.
	//
	// Set on remasters and expanded games as well as DLC — Skyrim Special
	// Edition points at Skyrim — so this alone never means "add-on". Only
	// kind dlc and expansion are treated that way.
	ParentIGDBID *int `json:"parentIgdbId" gorm:"column:parent_igdb_id"`
	// IGDB's short description. A plain string rather than a pointer, matching
	// Image: the catalog stores "" for absent, and the API reports "".
	Summary     string    `json:"summary"`
	ReleaseDate time.Time `json:"releaseDate"`

	// Stored. Loaded with Preload("Tags") and never serialised directly.
	Tags []*Tag `json:"-" gorm:"many2many:games_tags"`

	// Transport only, filled by SplitTags. Non-nil so JSON carries [] rather
	// than null — the same lesson as GetUserGames, where a nil slice made an
	// empty list crash the frontend.
	Genres    []*Tag `json:"genres" gorm:"-"`
	Themes    []*Tag `json:"themes" gorm:"-"`
	GameModes []*Tag `json:"gameModes" gorm:"-"`

	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

// SplitTags fans the stored tags out into the three transport fields.
//
// Called by every read path after Preload. Forgetting it is a silent bug: the
// game serialises with three empty arrays and looks like a game with no genres
// rather than a loading mistake.
func (g *Game) SplitTags() {
	g.Genres = []*Tag{}
	g.Themes = []*Tag{}
	g.GameModes = []*Tag{}

	for _, t := range g.Tags {
		switch t.Facet {
		case "genre":
			g.Genres = append(g.Genres, t)
		case "theme":
			g.Themes = append(g.Themes, t)
		case "game_mode":
			g.GameModes = append(g.GameModes, t)
		}
	}
}

// FromIGDB converts a fetched IGDB game into a catalog row.
//
// Here rather than in cmd/api because two binaries build catalog rows now: the
// API when somebody adds a game, and cmd/backfill when refreshing the lot. Two
// copies would drift the first time a field is added — which is exactly the
// kind of miss that left parent_igdb_id out of the upsert.
func FromIGDB(g igdb.Game) *Game {
	game := &Game{
		IGDBID:       g.IGDBID,
		Title:        g.Title,
		Kind:         g.Kind,
		ParentIGDBID: g.ParentIGDBID,
	}
	if g.Image != nil {
		game.Image = *g.Image
	}
	if g.Summary != nil {
		game.Summary = *g.Summary
	}
	if g.ReleaseDate != nil {
		// Parsed back from the YYYY-MM-DD the client formatted. Storing the
		// zero time for an unreleased game is fine: release_date is only ever
		// displayed.
		if t, err := time.Parse("2006-01-02", *g.ReleaseDate); err == nil {
			game.ReleaseDate = t
		}
	}

	for _, t := range g.Genres {
		game.Tags = append(game.Tags, &Tag{Facet: "genre", IGDBID: t.IGDBID, Name: t.Name})
	}
	for _, t := range g.Themes {
		game.Tags = append(game.Tags, &Tag{Facet: "theme", IGDBID: t.IGDBID, Name: t.Name})
	}
	for _, t := range g.GameModes {
		game.Tags = append(game.Tags, &Tag{Facet: "game_mode", IGDBID: t.IGDBID, Name: t.Name})
	}

	return game
}
