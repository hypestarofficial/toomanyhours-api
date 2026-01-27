package repository

import (
	"database/sql"
	"toomanyhours-api/internal/models"
)

type DatabaseRepo interface {
	Connection() *sql.DB
	AllGames() ([]*models.Game, error)
}