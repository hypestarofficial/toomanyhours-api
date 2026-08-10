package models

import "time"

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
	ParentIGDBID *int      `json:"parentIgdbId" gorm:"column:parent_igdb_id"`
	ReleaseDate  time.Time `json:"releaseDate"`

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
