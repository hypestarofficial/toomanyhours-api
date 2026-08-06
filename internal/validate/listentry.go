package validate

import (
	"errors"
	"math"
	"strings"
	"unicode/utf8"
)

// ErrRange is separate from ErrLength: a rating is not too long, it is outside
// the allowed values, and the handler's message to the user differs.
var ErrRange = errors.New("out of range")

// reviewMaxRunes is several paragraphs — far more than the couple of sentences
// the product aims at, while still bounding what one row can hold.
const reviewMaxRunes = 2000

// CategoryFinished is the one category that may hold a rating or a review.
// Exported because the rule below turns on this exact string; a second spelling
// of it is how the map and the rule would drift apart.
const CategoryFinished = "finished"

// categories are the stored values. They are snake_case because the database
// CHECK constraint is the source of truth and SQL convention wins; the
// frontend enum matches rather than mapping between two spellings.
var categories = map[string]struct{}{
	CategoryFinished:    {},
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

// ResultingCategory returns the category an entry will have once a patch is
// applied: the patched value when the request names one, otherwise the value
// the row already holds.
//
// The distinction is the whole rule. Finishing a game moves it and rates it in
// one request, so when that request is judged the row is still
// currently_playing — checking the current category would reject the single
// flow this exists to allow.
func ResultingCategory(current string, patch *string) string {
	if patch != nil {
		return *patch
	}
	return current
}

// RatingAllowed reports whether a rating or review may be written for an entry
// in this category.
//
// Gates writes only. A rating already stored on a row is kept when that row
// leaves finished — category is a column precisely so a move preserves it, and
// replaying a game should not cost you the review you wrote about it. That is
// also why this rule cannot be a Postgres CHECK constraint: the constraint
// would reject the move.
func RatingAllowed(category string) bool {
	return category == CategoryFinished
}

// Rating validates an optional half-step score between 0.5 and 10. A nil rating
// is valid and means unrated: VISION.md keeps rating optional because a list
// with a few scored standouts reads better than one where every entry has a
// dutiful number.
//
// Zero is rejected here. The API uses 0 as the "clear my rating" sentinel, and
// the handler converts it before validating — so a 0 reaching this function is
// a bug rather than a request.
func Rating(raw *float64) error {
	if raw == nil {
		return nil
	}
	if *raw < 0.5 || *raw > 10 {
		return ErrRange
	}
	// Half-steps only, matching the database CHECK. Doubling is exact for every
	// value the control can produce — halves are the one fraction binary
	// floating point always represents exactly — so this is a precise test
	// rather than one needing a tolerance.
	if doubled := *raw * 2; doubled != math.Trunc(doubled) {
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
