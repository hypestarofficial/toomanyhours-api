package igdb

// IGDB's game_type ids, needed for the exclusion clause. The names come back
// from the API; these numbers are what the query filters on.
const (
	typeMod       = 5
	typeFork      = 12
	typePackAddon = 13
	typeUpdate    = 14
)

// kindSlugs maps IGDB's display names to values worth storing.
//
// The slug is stored rather than IGDB's integer because a bare 13 in a column
// means nothing to whoever reads it next — and IGDB has already renumbered this
// concept once, deprecating `category` in favour of `game_type`.
var kindSlugs = map[string]string{
	"Main Game":            "main_game",
	"DLC":                  "dlc",
	"Expansion":            "expansion",
	"Bundle":               "bundle",
	"Standalone Expansion": "standalone_expansion",
	"Mod":                  "mod",
	"Episode":              "episode",
	"Season":               "season",
	"Remake":               "remake",
	"Remaster":             "remaster",
	"Expanded Game":        "expanded_game",
	"Port":                 "port",
	"Fork":                 "fork",
	"Pack / Addon":         "pack_addon",
	"Update":               "update",
}

// kindSlug converts an IGDB game_type name to a stored value. Anything
// unrecognised becomes "unknown" rather than an error: a new IGDB type must not
// be able to fail an import.
func kindSlug(name string) string {
	if slug, ok := kindSlugs[name]; ok {
		return slug
	}
	return "unknown"
}
