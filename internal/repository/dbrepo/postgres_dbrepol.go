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

// GetGames returns catalog games, optionally filtered by title and genres.
//
// A non-zero excludeUserID subtracts the games already in that user's list.
// Opt-in rather than always-on: this endpoint describes the catalog, and a
// caller asking what games exist should not silently get a personalised subset.
func (m *PostgresDBRepo) GetGames(ctx context.Context, title string, genreIDs []int, excludeUserID int) ([]*models.Game, error) {
	var games []*models.Game

	query := m.GormDB.WithContext(ctx).Model(&models.Game{})

	if title != "" {
		query = query.Where("title LIKE ?", "%"+title+"%")
	}

	if len(genreIDs) > 0 {
		subQuery := m.GormDB.Select("game_id").Table("games_genres").Where("genre_id IN (?)", genreIDs)
		query = query.Where("id IN (?)", subQuery)
	}

	if excludeUserID > 0 {
		// user_games.game_id is NOT NULL, so NOT IN has no null-semantics trap.
		owned := m.GormDB.Select("game_id").Table("user_games").Where("user_id = ?", excludeUserID)
		query = query.Where("id NOT IN (?)", owned)
	}

	result := query.Preload("Genres").
		Order("title").
		Find(&games)

	if result.Error != nil {
		return nil, result.Error
	}

	return games, nil
}

func (m *PostgresDBRepo) GetGenres(ctx context.Context) ([]*models.Genre, error) {
	var genres []*models.Genre

	result := m.GormDB.WithContext(ctx).
		Order("genre").
		Find(&genres)
	
	if result.Error != nil {
		return nil, result.Error
	}

	return genres, nil
}

func (m *PostgresDBRepo) GetGameByGameId(ctx context.Context, id int) (*models.Game, error) {
	var game models.Game

	result := m.GormDB.WithContext(ctx).
		Preload("Genres").
		First(&game, id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, sql.ErrNoRows
		}
		return nil, result.Error
	}

	return &game, nil
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

func (m *PostgresDBRepo) PostGameToGames(ctx context.Context, game models.Game) (int, error) {
	var existingGame models.Game

	err := m.GormDB.WithContext(ctx).First(&existingGame, game.ID).Error

	if err == nil {
		return existingGame.ID, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	result := m.GormDB.WithContext(ctx).
		Create(&game)

	if result.Error != nil {
		return 0, result.Error
	}

	return game.ID, nil
}

func (m *PostgresDBRepo) PostGameGenres(ctx context.Context, id int, genreIDs []int) error {
	var game models.Game
	game.ID = id

	// First, clear all existing genre associations
	err := m.GormDB.WithContext(ctx).
		Model(&game).
		Association("Genres").
		Clear()

	if err != nil {
		return err
	}

	// Then, add the new genres
	if len(genreIDs) > 0 {
		var genres []models.Genre
		for _, gid := range genreIDs {
			genres = append(genres, models.Genre{ID: gid})
		}

		err = m.GormDB.WithContext(ctx).
			Model(&game).
			Association("Genres").
			Append(genres)

		if err != nil {
			return err
		}
	}

	return nil
}

func (m* PostgresDBRepo) PutGameByGameId(ctx context.Context, game models.Game) error {
	result := m.GormDB.WithContext(ctx).
		Model(&game).
		Select("Title", "Image", "ReleaseDate").
		Updates(game)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("Game not found")
	}

	return nil
}

func (m *PostgresDBRepo) DeleteGameByGameId(ctx context.Context, id int) error {
	result := m.GormDB.WithContext(ctx).
		Unscoped().
		Delete(&models.Game{}, id)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("Game not found")
	}

	return nil
}