package repository

import (
	"context"
	"database/sql"
	"toomanyhours-api/internal/models"
)

type DatabaseRepo interface {
	Connection() *sql.DB

	// Games

	// Genres
	GetGenres(ctx context.Context) ([]*models.Tag, error)
	AllGameIGDBIDs(ctx context.Context) ([]int, error)
	GamesByIGDBIDs(ctx context.Context, igdbIDs []int) (map[int]int, error)
	ImportGames(ctx context.Context, games []*models.Game) (map[int]int, error)

	// Users
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	GetUserByUsername(ctx context.Context, username string) (*models.User, error)
	GetUserByID(ctx context.Context, id int) (*models.User, error)
	CreateUser(ctx context.Context, user *models.User) error
	UpdateUser(ctx context.Context, user *models.User) error

	// Games Genres

	// User Games (the list)
	GetUserGames(ctx context.Context, userID int) ([]*models.UserGame, error)
	AddUserGame(ctx context.Context, userID, gameID int, category string, rating *float64, review *string) (*models.UserGame, error)
	GetUserGameCategory(ctx context.Context, userID, gameID int) (string, error)
	UpdateUserGame(ctx context.Context, userID, gameID int, upd models.UserGameUpdate) (*models.UserGame, error)
	DeleteUserGame(ctx context.Context, userID, gameID int) error

	// Refresh tokens
	CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error
	GetRefreshToken(ctx context.Context, jti string) (*models.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, jti string) error
	FamilyHasActiveToken(ctx context.Context, familyID string) (bool, error)
	RevokeRefreshFamily(ctx context.Context, familyID string) error
	DeleteExpiredRefreshTokens(ctx context.Context) error
}
