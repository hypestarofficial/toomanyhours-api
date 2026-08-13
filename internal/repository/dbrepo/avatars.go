package dbrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"toomanyhours-api/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SetUserAvatar stores or replaces a user's photo.
//
// An upsert rather than a delete and insert: a user has at most one, and the
// conflict target is the primary key. The update list names every column the
// server owns — a field missing from such a list is silently ignored on
// replace, which is how parent_igdb_id and bio were both lost before.
func (m *PostgresDBRepo) SetUserAvatar(ctx context.Context, userID int, bytes []byte, hash string) error {
	row := &models.UserAvatar{UserID: userID, Bytes: bytes, Hash: hash, UpdatedAt: time.Now()}

	return m.GormDB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"bytes", "hash", "updated_at"}),
	}).Create(row).Error
}

// GetUserAvatar returns a user's photo, or sql.ErrNoRows when there is none.
//
// sql.ErrNoRows rather than GORM's own, matching GetUserByEmail: handlers here
// already branch on it.
func (m *PostgresDBRepo) GetUserAvatar(ctx context.Context, userID int) (*models.UserAvatar, error) {
	var avatar models.UserAvatar

	err := m.GormDB.WithContext(ctx).Where("user_id = ?", userID).First(&avatar).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	return &avatar, nil
}

// DeleteUserAvatar removes a user's photo. Removing one that is not there is
// not an error: the caller wanted no avatar, and there is none.
func (m *PostgresDBRepo) DeleteUserAvatar(ctx context.Context, userID int) error {
	return m.GormDB.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.UserAvatar{}).Error
}
