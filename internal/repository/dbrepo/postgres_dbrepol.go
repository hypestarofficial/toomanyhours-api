package dbrepo

import (
	"context"
	"database/sql"
	"time"
	"toomanyhours-api/internal/models"
)

type PostgresDBRepo struct {
	DB *sql.DB
}

const dbTimeout = time.Second * 3

func (m *PostgresDBRepo) Connection() *sql.DB {
	return m.DB
}

func (m *PostgresDBRepo) GetGames(title string) ([]*models.Game, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// Logic: If $1 is empty (''), the first condition is true for all rows.
	// Otherwise, it checks if the title matches the pattern.
	query := `
		SELECT 
			id, title, coalesce(image, ''), release_date
		FROM
			games
		WHERE 
			$1 = '' OR title ILIKE '%' || $1 || '%'
		ORDER BY
			title
	`

	rows, err := m.DB.QueryContext(ctx, query, title)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var games []*models.Game
	for rows.Next() {
		var game models.Game
		err := rows.Scan(
			&game.ID,
			&game.Title,
			&game.Image,
			&game.ReleaseDate,
		)
		if err != nil {
			return nil, err
		}
		games = append(games, &game)
	}

	return games, nil
}

func (m *PostgresDBRepo) GetGenres() ([]*models.Genre, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		SELECT 
			id, genre
		FROM
			genres
		ORDER BY
			genre
	`

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var genres []*models.Genre
	for rows.Next() {
		var genre models.Genre
		err := rows.Scan(
			&genre.ID,
			&genre.Genre,
		)
		if err != nil {
			return nil, err
		}
		genres = append(genres, &genre)
	}

	return genres, nil
}

func (m *PostgresDBRepo) GetGameByGameId(id int) (*models.Game, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		SELECT
			id, title, coalesce(image, ''), release_date
		FROM
			games
		WHERE id = $1
	`

	row := m.DB.QueryRowContext(ctx, query, id)

	var game models.Game

	err := row.Scan(
		&game.ID,
		&game.Title,
		&game.Image,
		&game.ReleaseDate,
	)

	if err != nil {
		return nil, err
	}

	// get genres for the game
	query = `
		SELECT
			g.id, g.genre
		FROM
			games_genres gg LEFT JOIN genres g ON gg.genre_id = g.id
		WHERE
			gg.game_id = $1
		ORDER BY
			g.genre
	`
	rows, err := m.DB.QueryContext(ctx, query, id)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	defer rows.Close()

	var genres []*models.Genre
	
	for rows.Next() {
		var g models.Genre
		err := rows.Scan(
			&g.ID,
			&g.Genre,
		)
		if err != nil {
			return nil, err
		}

		genres = append(genres, &g)
	}

	game.Genres = genres

	return &game, nil
}

func (m *PostgresDBRepo) GetUserByEmail(email string) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		SELECT
			id, username, email, password, created_at, updated_at
		FROM
			users
		WHERE
			email = $1
	`

	var user models.User
	row := m.DB.QueryRowContext(ctx, query, email)

	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (m *PostgresDBRepo) GetUserByID(id int) (*models.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		SELECT
			id, username, email, password, created_at, updated_at
		FROM
			users
		WHERE
			id = $1
	`

	var user models.User
	row := m.DB.QueryRowContext(ctx, query, id)

	err := row.Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Password,
		&user.CreatedAt,
		&user.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}

	return &user, nil
}