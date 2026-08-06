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
	ptr := func(n int) *int { return &n }

	tests := []struct {
		name    string
		input   *int
		wantErr error
	}{
		{"nil is unrated and valid", nil, nil},
		{"one", ptr(1), nil},
		{"ten", ptr(10), nil},
		{"zero is not a rating", ptr(0), ErrRange},
		{"eleven", ptr(11), ErrRange},
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
		if _, err := Review(ptr(strings.Repeat("a", 2000))); err != nil {
			t.Fatalf("2000 runes should be allowed, got %v", err)
		}
	})

	t.Run("over the cap", func(t *testing.T) {
		if _, err := Review(ptr(strings.Repeat("a", 2001))); !errors.Is(err, ErrLength) {
			t.Fatalf("2001 runes error = %v, want ErrLength", err)
		}
	})

	// The cap counts characters, not bytes: a review in Japanese or with
	// emoji must not be rejected at a third of the length an English one gets.
	t.Run("cap counts runes, not bytes", func(t *testing.T) {
		if _, err := Review(ptr(strings.Repeat("あ", 2000))); err != nil {
			t.Fatalf("2000 multibyte runes should be allowed, got %v", err)
		}
	})
}
