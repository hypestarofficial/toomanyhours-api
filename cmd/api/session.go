package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
	"toomanyhours-api/internal/models"
)

// newTokenID returns a random 128-bit identifier, hex encoded.
//
// crypto/rand rather than a UUID library: the same unguessability with no
// dependency. math/rand would be a real bug here — these are credentials, and
// a predictable jti is a guessable session.
func newTokenID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// issueSession records a refresh token and returns the signed pair for it.
//
// An empty familyID starts a new family, which is what login and registration
// do. Passing an existing one continues a rotation chain, so every token
// descended from one login can be revoked together.
//
// The row is written before the token is signed, so a client can never hold a
// token that references a row which does not exist.
func (app *application) issueSession(ctx context.Context, user *models.User, familyID string) (TokenPairs, error) {
	jti, err := newTokenID()
	if err != nil {
		return TokenPairs{}, err
	}

	if familyID == "" {
		familyID, err = newTokenID()
		if err != nil {
			return TokenPairs{}, err
		}
	}

	row := &models.RefreshToken{
		JTI:      jti,
		FamilyID: familyID,
		UserID:   user.ID,
		// Fresh 24 hours per successor, so a session stays alive while it is
		// used and dies a day after it stops. Reuse detection is what bounds a
		// stolen token that an attacker keeps rotating.
		ExpiresAt: time.Now().Add(app.auth.RefreshTokenExpiry),
	}

	if err := app.DB.CreateRefreshToken(ctx, row); err != nil {
		return TokenPairs{}, err
	}

	return app.auth.GenerateTokenPair(&jwtUser{ID: user.ID, Username: user.Username}, jti)
}
