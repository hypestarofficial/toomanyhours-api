package validate

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// ErrRange is separate from ErrLength: a rating is not too long, it is outside
// the allowed values, and the handler's message to the user differs.
var ErrRange = errors.New("out of range")

// reviewMaxRunes is several paragraphs — far more than the couple of sentences
// the product aims at, while still bounding what one row can hold.
const reviewMaxRunes = 2000

// categories are the stored values. They are snake_case because the database
// CHECK constraint is the source of truth and SQL convention wins; the
// frontend enum matches rather than mapping between two spellings.
var categories = map[string]struct{}{
	"finished":          {},
	"currently_playing": {},
	"want_to_play":      {},
}

// Category validates a list category and returns the normalized value to store.
func Category(raw string) (string, error) {
	category := strings.ToLower(strings.TrimSpace(raw))
	if _, ok := categories[category]; !ok {
		return "", ErrFormat
	}
	return category, nil
}

// Rating validates an optional 1-10 score. A nil rating is valid and means
// unrated: VISION.md keeps rating optional because a list with a few scored
// standouts reads better than one where every entry has a dutiful number.
//
// Zero is rejected here. The API uses 0 as the "clear my rating" sentinel, and
// the handler converts it before validating — so a 0 reaching this function is
// a bug rather than a request.
func Rating(raw *int) error {
	if raw == nil {
		return nil
	}
	if *raw < 1 || *raw > 10 {
		return ErrRange
	}
	return nil
}

// Review normalizes an optional review, returning the value to store. A blank
// review collapses to nil so that "cleared" has one representation in the
// database rather than two that queries would have to remember to check for.
func Review(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}

	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil, nil
	}

	// Runes, not bytes: the cap is about how much someone wrote, and counting
	// bytes would give a review in Japanese a third of the allowance.
	if utf8.RuneCountInString(trimmed) > reviewMaxRunes {
		return nil, ErrLength
	}

	return &trimmed, nil
}
