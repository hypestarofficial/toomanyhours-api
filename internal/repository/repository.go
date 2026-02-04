package repository

import (
	"database/sql"
	"toomanyhours-api/internal/models"
)

type DatabaseRepo interface {
	Connection() *sql.DB

	// Games
	GetGames(title string, genreIDs []int) ([]*models.Game, error)
	GetGameByGameId(id int) (*models.Game, error)
	PostGameToGames(game models.Game) (int, error)
	PutGameByGameId(game models.Game) error
	DeleteGameByGameId(id int) error

	// Genres
	GetGenres() ([]*models.Genre, error)

	// Users
	GetUserByEmail(email string) (*models.User, error)
	GetUserByID(id int) (*models.User, error)

	// Games Genres
	PostGameGenres(id int, genreIDs []int) error
}