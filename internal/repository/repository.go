package repository

import (
	"context"
	"database/sql"
	"toomanyhours-api/internal/models"
)

type DatabaseRepo interface {
	Connection() *sql.DB

	// Games
	GetGames(ctx context.Context, title string, genreIDs []int, excludeUserID int) ([]*models.Game, error)
	GetGameByGameId(ctx context.Context, id int) (*models.Game, error)
	PostGameToGames(ctx context.Context, game models.Game) (int, error)
	PutGameByGameId(ctx context.Context, game models.Game) error
	DeleteGameByGameId(ctx context.Context, id int) error

	// Genres
	GetGenres(ctx context.Context) ([]*models.Genre, error)

	// Users
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByID(ctx context.Context, id int) (*models.User, error)
	CreateUser(ctx context.Context, user *models.User) error
	UpdateUser(ctx context.Context, user *models.User) error

	// Games Genres
	PostGameGenres(ctx context.Context, id int, genreIDs []int) error

	// User Games (the list)
	GetUserGames(ctx context.Context, userID int) ([]*models.UserGame, error)
	AddUserGames(ctx context.Context, userID int, gameIDs []int, category string) ([]*models.UserGame, error)
	GetUserGameCategory(ctx context.Context, userID, gameID int) (string, error)
	UpdateUserGame(ctx context.Context, userID, gameID int, upd models.UserGameUpdate) (*models.UserGame, error)
	DeleteUserGame(ctx context.Context, userID, gameID int) error
	GamesExist(ctx context.Context, gameIDs []int) (bool, error)

	// Refresh tokens
	CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error
	GetRefreshToken(ctx context.Context, jti string) (*models.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, jti string) error
	FamilyHasActiveToken(ctx context.Context, familyID string) (bool, error)
	RevokeRefreshFamily(ctx context.Context, familyID string) error
	DeleteExpiredRefreshTokens(ctx context.Context) error
}