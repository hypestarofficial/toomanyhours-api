package igdb

import (
	"strings"
	"unicode"
)

// escapeSearchTerm makes a user's query safe to interpolate into an Apicalypse
// string literal.
//
// The query goes into `search "<term>";`, so an unescaped double quote closes
// the literal early and everything after it is read as query syntax. The blast
// radius is smaller than SQL injection — the worst case is a malformed query,
// or one that asks IGDB for different fields — but it is the same shape of bug,
// and "smaller" is not a reason to leave it.
//
// Escapes rather than filters. Plenty of real titles contain quotes, colons and
// apostrophes ("Marvel's Spider-Man", "Uncharted 4: A Thief's End"), and
// stripping them would silently return the wrong results, which is harder to
// notice than an error.
func escapeSearchTerm(raw string) string {
	var b strings.Builder
	b.Grow(len(raw))

	for _, r := range raw {
		switch {
		case r == '\\' || r == '"':
			b.WriteRune('\\')
			b.WriteRune(r)
		case r == '\n' || r == '\r' || r == '\t':
			// Whitespace in a title is meaningful as a separator, so these
			// become spaces rather than vanishing and joining two words.
			b.WriteRune(' ')
		case unicode.IsControl(r):
			// Everything else unprintable is dropped: it cannot be part of a
			// title and has no business in a query.
		default:
			b.WriteRune(r)
		}
	}

	return b.String()
}
