package igdb

import "testing"

func TestEscapeSearchTerm(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text is untouched", "witcher", "witcher"},
		// Real titles contain these. Filtering them out would silently mangle
		// the search, which is why this escapes rather than strips.
		{"apostrophes survive", "Marvel's Spider-Man", "Marvel's Spider-Man"},
		{"colons survive", "Uncharted 4: A Thief's End", "Uncharted 4: A Thief's End"},
		{"accents survive", "Pokémon", "Pokémon"},
		{"a double quote is escaped", `say "hi"`, `say \"hi\"`},
		{"a backslash is escaped", `back\slash`, `back\\slash`},
		// The injection case. Unescaped, this would close the search literal and
		// start a new clause, asking IGDB for fields the caller never should.
		{
			"a query that tries to close the literal",
			`x"; fields *; limit 500;`,
			`x\"; fields *; limit 500;`,
		},
		{"newlines become spaces", "two\nlines", "two lines"},
		{"tabs become spaces", "two\ttabs", "two tabs"},
		{"other control characters are dropped", "null\x00byte", "nullbyte"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := escapeSearchTerm(tc.input); got != tc.want {
				t.Fatalf("escapeSearchTerm(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
