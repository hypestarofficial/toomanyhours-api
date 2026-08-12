package models

import (
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User mirrors the users table. Schema is owned by migrations/, not by these
// tags: the previous `gorm:"unique"` tags never did anything, because the
// schema came from a SQL file and AutoMigrate is never called. Uniqueness is
// enforced by the users_username_lower_idx / users_email_lower_idx indexes.
type User struct {
	ID         int    `json:"id" gorm:"primaryKey"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Password   string `json:"-"`
	Visibility string `json:"visibility"`
	// Nullable: a bio nobody has written is absent, not empty. Capped at 500
	// characters by users_bio_length and by validate.Bio, which count the same
	// thing — characters and runes are the same unit.
	Bio       *string   `json:"bio"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// APIUser is the shape sent to clients. It exists so Password can never be
// serialized by accident.
type APIUser struct {
	ID         int       `json:"id"`
	Username   string    `json:"username"`
	Email      string    `json:"email"`
	Visibility string    `json:"visibility"`
	Bio        *string   `json:"bio"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (u *User) PasswordMatches(plainTextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(plainTextPassword))
	if err != nil {
		switch {
		case errors.Is(err, bcrypt.ErrMismatchedHashAndPassword):
			return false, nil
		default:
			return false, err
		}
	}

	return true, nil
}
