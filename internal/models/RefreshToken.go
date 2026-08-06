package models

import "time"

// RefreshToken is one issued refresh token.
//
// The JWT handed to the client carries only JTI. Everything that decides
// whether the token still counts — who it belongs to, when it expires, whether
// it has been consumed — lives here, and that inversion is the whole point:
// a self-contained token cannot be revoked, a lookup key can.
//
// Every field is json:"-". This never belongs in a response, and a struct that
// cannot be serialized cannot be leaked by a careless c.JSON.
type RefreshToken struct {
	ID int `json:"-" gorm:"primaryKey"`
	// Explicit column tag: relying on the naming strategy to render an
	// initialism is a coin flip, and getting it wrong fails at runtime.
	JTI       string     `json:"-" gorm:"column:jti"`
	FamilyID  string     `json:"-"`
	UserID    int        `json:"-"`
	ExpiresAt time.Time  `json:"-"`
	RevokedAt *time.Time `json:"-"`
	CreatedAt time.Time  `json:"-"`
}
