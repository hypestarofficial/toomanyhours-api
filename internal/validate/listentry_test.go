package validate

import (
	"errors"
	"strings"
	"testing"
)

func TestCategory(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{"finished", "finished", "finished", nil},
		{"currently playing", "currently_playing", "currently_playing", nil},
		{"want to play", "want_to_play", "want_to_play", nil},
		{"normalizes case and space", "  Finished  ", "finished", nil},
		{"camelCase is not accepted", "currentlyPlaying", "", ErrFormat},
		{"invented category", "abandoned", "", ErrFormat},
		{"empty", "", "", ErrFormat},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Category(tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Category(%q) error = %v, want %v", tc.input, err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("Category(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestRating(t *testing.T) {
	ptr := func(f float64) *float64 { return &f }

	tests := []struct {
		name    string
		input   *float64
		wantErr error
	}{
		{"nil is unrated and valid", nil, nil},
		{"the lowest half step", ptr(0.5), nil},
		{"a whole star", ptr(7), nil},
		{"a half step", ptr(6.5), nil},
		{"the top of the scale", ptr(10), nil},
		// The reason this function is not just a range check. A quarter step is
		// inside the range and still not a rating anything can produce.
		{"a quarter step", ptr(6.25), ErrRange},
		{"a tenth", ptr(0.3), ErrRange},
		{"zero is not a rating", ptr(0), ErrRange},
		{"above the scale", ptr(10.5), ErrRange},
		{"negative", ptr(-3), ErrRange},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := Rating(tc.input); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Rating() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestReview(t *testing.T) {
	ptr := func(s string) *string { return &s }

	t.Run("nil stays nil", func(t *testing.T) {
		got, err := Review(nil)
		if err != nil || got != nil {
			t.Fatalf("Review(nil) = %v, %v; want nil, nil", got, err)
		}
	})

	t.Run("blank normalizes to nil so cleared has one representation", func(t *testing.T) {
		got, err := Review(ptr("   \n\t "))
		if err != nil || got != nil {
			t.Fatalf("Review(blank) = %v, %v; want nil, nil", got, err)
		}
	})

	t.Run("trims surrounding whitespace", func(t *testing.T) {
		got, err := Review(ptr("  loved it  "))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got == nil || *got != "loved it" {
			t.Fatalf("Review() = %v, want \"loved it\"", got)
		}
	})

	t.Run("at the cap", func(t *testing.T) {
		if _, err := Review(ptr(strings.Repeat("a", 8000))); err != nil {
			t.Fatalf("8000 runes should be allowed, got %v", err)
		}
	})

	t.Run("over the cap", func(t *testing.T) {
		if _, err := Review(ptr(strings.Repeat("a", 8001))); !errors.Is(err, ErrLength) {
			t.Fatalf("2001 runes error = %v, want ErrLength", err)
		}
	})

	// The cap counts characters, not bytes: a review in Japanese or with
	// emoji must not be rejected at a third of the length an English one gets.
	t.Run("cap counts runes, not bytes", func(t *testing.T) {
		if _, err := Review(ptr(strings.Repeat("あ", 8000))); err != nil {
			t.Fatalf("8000 multibyte runes should be allowed, got %v", err)
		}
	})
}

func TestResultingCategory(t *testing.T) {
	ptr := func(s string) *string { return &s }

	tests := []struct {
		name    string
		current string
		patch   *string
		want    string
	}{
		{"no patch keeps the current category", "finished", nil, "finished"},
		{"a patch wins over the current category", "currently_playing", ptr("finished"), "finished"},
		{"moving out of finished", "finished", ptr("want_to_play"), "want_to_play"},
		{"a patch that changes nothing", "finished", ptr("finished"), "finished"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResultingCategory(tc.current, tc.patch); got != tc.want {
				t.Fatalf("ResultingCategory(%q, %v) = %q, want %q", tc.current, tc.patch, got, tc.want)
			}
		})
	}
}

func TestRatingAllowed(t *testing.T) {
	tests := []struct {
		category string
		want     bool
	}{
		{"finished", true},
		{"currently_playing", false},
		{"want_to_play", false},
		{"", false},
	}

	for _, tc := range tests {
		t.Run(tc.category, func(t *testing.T) {
			if got := RatingAllowed(tc.category); got != tc.want {
				t.Fatalf("RatingAllowed(%q) = %v, want %v", tc.category, got, tc.want)
			}
		})
	}
}

// The flow the rule exists to allow, and the one that breaks if somebody later
// "simplifies" the handler to check the entry's current category. Finishing a
// game moves it and rates it in a single request, so at the moment the request
// is judged the row is still currently_playing.
func TestFinishInOneRequestIsAllowed(t *testing.T) {
	finished := "finished"

	if !RatingAllowed(ResultingCategory("currently_playing", &finished)) {
		t.Fatal("finishing a currently_playing entry with a rating must be allowed")
	}

	// And the inverse: the same entry without the category change must not be.
	if RatingAllowed(ResultingCategory("currently_playing", nil)) {
		t.Fatal("rating a currently_playing entry without finishing it must be rejected")
	}
}
