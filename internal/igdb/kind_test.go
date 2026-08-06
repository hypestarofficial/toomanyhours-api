package igdb

import "testing"

func TestKindSlug(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"Main Game", "main_game"},
		{"DLC", "dlc"},
		{"Expansion", "expansion"},
		{"Bundle", "bundle"},
		{"Standalone Expansion", "standalone_expansion"},
		{"Mod", "mod"},
		{"Episode", "episode"},
		{"Season", "season"},
		{"Remake", "remake"},
		{"Remaster", "remaster"},
		{"Expanded Game", "expanded_game"},
		{"Port", "port"},
		{"Fork", "fork"},
		{"Pack / Addon", "pack_addon"},
		{"Update", "update"},
		// IGDB has already renumbered and renamed this concept once — `category`
		// was deprecated in favour of `game_type`. A new value must degrade to
		// something storable rather than crash an import.
		{"Some Future Type", "unknown"},
		{"", "unknown"},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			if got := kindSlug(tc.in); got != tc.want {
				t.Fatalf("kindSlug(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
