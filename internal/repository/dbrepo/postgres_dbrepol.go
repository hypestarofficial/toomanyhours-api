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
		// Tag ids now, not genre ids. The parameter keeps its name because the
		// query string the frontend sends is still genreIds; renaming that is
		// cycle 3's business along with the rest of that surface.
		subQuery := m.GormDB.Select("game_id").Table("games_tags").Where("tag_id IN (?)", genreIDs)
		query = query.Where("id IN (?)", subQuery)
	}

	if excludeUserID > 0 {
		// user_games.game_id is NOT NULL, so NOT IN has no null-semantics trap.
		owned := m.GormDB.Select("game_id").Table("user_games").Where("user_id = ?", excludeUserID)
		query = query.Where("id NOT IN (?)", owned)
	}

	result := query.Preload("Tags").
		Order("title").
		Find(&games)

	if result.Error != nil {
		return nil, result.Error
	}

	// Without this every game serialises with three empty arrays, which reads
	// as a game with no genres rather than a loading mistake.
	for _, g := range games {
		g.SplitTags()
	}

	return games, nil
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

func (m *PostgresDBRepo) GetGameByGameId(ctx context.Context, id int) (*models.Game, error) {
	var game models.Game

	result := m.GormDB.WithContext(ctx).
		Preload("Tags").
		First(&game, id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, sql.ErrNoRows
		}
		return nil, result.Error
	}

	game.SplitTags()

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
