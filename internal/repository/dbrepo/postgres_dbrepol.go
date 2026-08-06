package dbrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"
	"toomanyhours-api/internal/models"

	"gorm.io/gorm"
)

type PostgresDBRepo struct {
	GormDB *gorm.DB
}

const dbTimeout = time.Second * 3

func (m *PostgresDBRepo) Connection() *sql.DB {
	db, err := m.GormDB.DB()
	if err != nil {
		panic(err)
	}

	return db
}

// GetGenres returns the genre tags, for the filter dropdown. Themes and game
// modes live in the same table and are deliberately not returned: nothing
// filters by them yet.
func (m *PostgresDBRepo) GetGenres(ctx context.Context) ([]*models.Tag, error) {
	var genres []*models.Tag

	result := m.GormDB.WithContext(ctx).
		Where("facet = ?", "genre").
		Order("name").
		Find(&genres)

	if result.Error != nil {
		return nil, result.Error
	}

	// Empty rather than nil: the frontend maps over this.
	if genres == nil {
		genres = []*models.Tag{}
	}
	return genres, nil
}

func (m *PostgresDBRepo) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User

	result := m.GormDB.WithContext(ctx).
		Where("email = ?", email).
		First(&user)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, sql.ErrNoRows
		}
		return nil, result.Error
	}

	return &user, nil
}

func (m *PostgresDBRepo) GetUserByID(ctx context.Context, id int) (*models.User, error) {
	var user models.User

	result := m.GormDB.WithContext(ctx).
		First(&user, id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, sql.ErrNoRows
		}
		return nil, result.Error
	}

	return &user, nil
}

// CreateUser inserts a user. Uniqueness of username and email is enforced by
// the database, not by a prior SELECT: checking first and inserting second is a
// race in which two simultaneous registrations both observe the name as free.
// A unique violation surfaces as gorm.ErrDuplicatedKey, which requires
// TranslateError to be enabled on the GORM config.
func (m *PostgresDBRepo) CreateUser(ctx context.Context, user *models.User) error {
	return m.GormDB.WithContext(ctx).Create(user).Error
}

// UpdateUser persists username and visibility changes. Password and email are
// deliberately excluded from the Select: changing either needs its own flow,
// and listing them here would let a stray zero value blank them.
func (m *PostgresDBRepo) UpdateUser(ctx context.Context, user *models.User) error {
	result := m.GormDB.WithContext(ctx).
		Model(user).
		Select("Username", "Visibility").
		Updates(user)

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
