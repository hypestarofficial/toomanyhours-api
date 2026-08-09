package dbrepo

import (
	"context"
	"time"
	"toomanyhours-api/internal/models"

	"gorm.io/gorm"
)

// scopedToUser is the guard every method here goes through. Taking userID as a
// parameter and never accepting it from a request is what stops one user
// reading or editing another's list by changing an integer.
func (m *PostgresDBRepo) scopedToUser(ctx context.Context, userID int) *gorm.DB {
	return m.GormDB.WithContext(ctx).Where("user_id = ?", userID)
}

// GetUserGames returns every entry in a user's list, newest first.
//
// Ordered by created_at rather than updated_at: ordering by the latter would
// make dragging a game silently reshuffle the list under the cursor.
func (m *PostgresDBRepo) GetUserGames(ctx context.Context, userID int) ([]*models.UserGame, error) {
	// An empty slice, never nil. A nil slice marshals to JSON null, and the
	// frontend maps over this array — so a new user with no games would get a
	// crash rather than three empty sections.
	entries := make([]*models.UserGame, 0)

	result := m.scopedToUser(ctx, userID).
		Preload("Game").
		// The nested preload is required, not belt-and-braces: GameCard renders
		// genre badges, and one level of Preload leaves Tags empty, which looks
		// like a styling bug rather than a query bug.
		Preload("Game.Tags").
		Order("created_at DESC").
		Find(&entries)

	if result.Error != nil {
		return nil, result.Error
	}

	// Tags are stored in one table; the three transport fields only exist once
	// this runs. Skipping it is the same silent failure as skipping the preload.
	for _, e := range entries {
		if e.Game != nil {
			e.Game.SplitTags()
		}
	}

	return entries, nil
}

// AddUserGame adds one game to a user's list.
//
// No ON CONFLICT clause: a game already in the list violates
// user_games_user_id_game_id_key, which GORM surfaces as gorm.ErrDuplicatedKey
// because the connection is opened with TranslateError. The caller turns that
// into a 409.
//
// The constraint is the authority rather than a pre-flight existence check:
// looking first and inserting second leaves a window where another tab inserts
// in between, and the constraint has no such window.
//
// Rating and review are written at insert time so a finished game arrives
// complete. Both may be nil; the handler has already checked that a non-nil
// one is allowed for this category.
func (m *PostgresDBRepo) AddUserGame(
	ctx context.Context,
	userID, gameID int,
	category string,
	rating *float64,
	review *string,
) (*models.UserGame, error) {
	row := &models.UserGame{
		UserID:   userID,
		GameID:   gameID,
		Category: category,
		Rating:   rating,
		Review:   review,
	}

	if err := m.GormDB.WithContext(ctx).Create(row).Error; err != nil {
		return nil, err
	}

	// Re-read so the response carries the game and its genres. The inserted
	// row holds only foreign keys.
	var entry models.UserGame

	result := m.scopedToUser(ctx, userID).
		Where("game_id = ?", gameID).
		Preload("Game").
		Preload("Game.Tags").
		First(&entry)

	if result.Error != nil {
		return nil, result.Error
	}

	// Preload fills Tags; SplitTags is what turns them into the genres, themes
	// and game modes the frontend renders. Forgetting it serialises three
	// empty arrays, which reads as a game with no genres.
	if entry.Game != nil {
		entry.Game.SplitTags()
	}

	return &entry, nil
}

// GetUserGameCategory returns just the category of one entry.
//
// Narrow on purpose. The rating rule needs one column, and loading the row with
// its game and its genres preloaded to read a single string would be the
// expensive way to ask. Returns gorm.ErrRecordNotFound when the entry is not in
// this user's list, matching UpdateUserGame so the handler can answer 404 the
// same way for both.
func (m *PostgresDBRepo) GetUserGameCategory(ctx context.Context, userID, gameID int) (string, error) {
	var entry models.UserGame

	// First is what produces gorm.ErrRecordNotFound on a miss. Pluck would leave
	// the string empty and return a nil error, so a missing entry would read as
	// an empty category and fail the rule with the wrong status code.
	err := m.scopedToUser(ctx, userID).
		Select("category").
		Where("game_id = ?", gameID).
		First(&entry).Error

	if err != nil {
		return "", err
	}
	return entry.Category, nil
}

// UpdateUserGame applies a PATCH to one entry, returning the updated row.
// Returns gorm.ErrRecordNotFound when the entry is not in this user's list —
// including when it exists but belongs to someone else, which the caller turns
// into a 404 rather than a 403 so the response does not confirm it exists.
func (m *PostgresDBRepo) UpdateUserGame(ctx context.Context, userID, gameID int, upd models.UserGameUpdate) (*models.UserGame, error) {
	updates := map[string]any{"updated_at": time.Now()}

	if upd.Category != nil {
		updates["category"] = *upd.Category
	}
	// The Set flags, not nil-ness, decide: a nil Rating with SetRating true is
	// an explicit "clear it", and must write NULL rather than be skipped.
	if upd.SetRating {
		updates["rating"] = upd.Rating
	}
	if upd.SetReview {
		updates["review"] = upd.Review
	}

	result := m.scopedToUser(ctx, userID).
		Model(&models.UserGame{}).
		Where("game_id = ?", gameID).
		Updates(updates)

	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var entry models.UserGame

	err := m.scopedToUser(ctx, userID).
		Where("game_id = ?", gameID).
		Preload("Game").
		Preload("Game.Tags").
		First(&entry).Error

	if err != nil {
		return nil, err
	}

	if entry.Game != nil {
		entry.Game.SplitTags()
	}

	return &entry, nil
}

// DeleteUserGame removes one entry from a user's list.
func (m *PostgresDBRepo) DeleteUserGame(ctx context.Context, userID, gameID int) error {
	result := m.scopedToUser(ctx, userID).
		Where("game_id = ?", gameID).
		Delete(&models.UserGame{})

	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
