package repository

import (
	"database/sql"
	"toomanyhours-api/internal/models"
)

type DatabaseRepo interface {
	Connection() *sql.DB
	AllGames() ([]*models.Game, error)
	GetUserByEmail(email string) (*models.User, error)
	GetUserByID(id int) (*models.User, error)
}