package repository

import (
	"database/sql"
	"toomanyhours-api/internal/models"
)

type DatabaseRepo interface {
	Connection() *sql.DB
	GetGames(title string) ([]*models.Game, error)
	GetGameByGameId(id int) (*models.Game, error)
	GetGenres() ([]*models.Genre, error)
	GetUserByEmail(email string) (*models.User, error)
	GetUserByID(id int) (*models.User, error)
}