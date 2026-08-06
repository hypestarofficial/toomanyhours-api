package models

import "time"

// UserGame is one entry in a user's list — the heart of the product. There is
// one row per (user, game), enforced by a unique constraint in migration
// 000003, which is why moving a game between categories preserves its rating.
//
// Schema lives in migrations/, not in these tags: AutoMigrate is never called.
type UserGame struct {
	ID int `json:"id" gorm:"primaryKey"`
	// json:"-" for the same reason User.Password is: a field that must never
	// leave the server is best made unable to. The client already knows who it
	// is, and accepting this value from a request would let anyone edit
	// anyone's list.
	UserID   int    `json:"-"`
	GameID   int    `json:"gameId"`
	Category string `json:"category"`
	// Pointers because absent and zero are different: no rating is not a
	// rating of nothing. float64 because the scale has half-steps — 6.5 stars
	// is a rating of 6.5, stored in a numeric(3,1) column since migration
	// 000005.
	Rating *float64 `json:"rating"`
	Review *string  `json:"review"`
	Hours  *int     `json:"hours"`
	// Populated on reads via Preload. The frontend renders the game's title,
	// cover and genres from here.
	Game      *Game     `json:"game,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// UserGameUpdate carries what a PATCH may change.
//
// The Set* flags exist because a nil Rating otherwise means two different
// things: "the client did not mention it" and "the client cleared it". Go's
// JSON decoder cannot tell an absent key from an explicit null, so the handler
// resolves the intent from the API's sentinel values (rating 0, review "")
// and states it here unambiguously.
type UserGameUpdate struct {
	Category  *string
	SetRating bool
	Rating    *float64
	SetReview bool
	Review    *string
}
