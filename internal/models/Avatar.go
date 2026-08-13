package models

import "time"

// UserAvatar is one user's profile photo: a 256x256 JPEG of about 20KB.
//
// Its own table rather than a column on users, because GORM selects every
// column by default — an avatar on users would ride along on every login and
// every profile read.
//
// Schema lives in migrations/, not in these tags: AutoMigrate is never called.
type UserAvatar struct {
	UserID    int       `json:"-" gorm:"primaryKey"`
	Bytes     []byte    `json:"-"`
	Hash      string    `json:"hash"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Named explicitly so a rename cannot silently change which table this reads.
func (UserAvatar) TableName() string { return "user_avatars" }
