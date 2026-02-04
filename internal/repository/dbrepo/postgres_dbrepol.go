package dbrepo

import (
	"context"
	"database/sql"
	"errors"
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

func (m *PostgresDBRepo) GetGames(title string, genreIDs []int) ([]*models.Game, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	query := `
		SELECT 
			id, title, coalesce(image, ''), release_date
		FROM
			games
		WHERE 
			($1 = '' OR title ILIKE '%' || $1 || '%')
	`
	
	args := []any{title}

	if len(genreIDs) > 0 {
		query += ` AND id IN (
			SELECT game_id 
			FROM games_genres 
			WHERE genre_id = ANY($2)
		)`
		args = append(args, genreIDs)
	}

	query += `
		ORDER BY
			title
	`

	rows, err := m.DB.QueryContext(ctx, query, args...)
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

	// for each game, fetch its genres from games_genres
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

	for _, game := range games {
		genreRows, err := m.DB.QueryContext(ctx, query, game.ID)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}

		var genres []*models.Genre
		for genreRows.Next() {
			var genre models.Genre
			if err := genreRows.Scan(
				&genre.ID,
				&genre.Genre,
			); err != nil {
				genreRows.Close()
				return nil, err
			}
			genres = append(genres, &genre)
		}
		genreRows.Close()

		game.Genres = genres
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

func (m *PostgresDBRepo) PostGameToGames(game models.Game) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	existingGame, err := m.GetGameByGameId(game.ID)
	if err == nil && existingGame != nil {
		return existingGame.ID, nil
	}
	if err != nil && err != sql.ErrNoRows {
		return 0, err
	}

	query := `
		INSERT INTO games (id, title, image, release_date)
		OVERRIDING SYSTEM VALUE
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	var insertedID int
	err = m.DB.QueryRowContext(ctx, query, game.ID, game.Title, game.Image, game.ReleaseDate).Scan(&insertedID)

	if err != nil {
		return 0, err
	}


	return game.ID, nil
}

func (m *PostgresDBRepo) PostGameGenres(id int, genreIDs []int) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	// Delete existing genre associations for this game
	stmt := `DELETE FROM games_genres WHERE game_id = $1`
	_, err := m.DB.ExecContext(ctx, stmt, id)
	if err != nil {
		return err
	}

	// Insert new genre associations
	for _, n := range genreIDs {
		stmt = `INSERT INTO games_genres (game_id, genre_id) VALUES ($1, $2)`
		_, err := m.DB.ExecContext(ctx, stmt, id, n)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m* PostgresDBRepo) PutGameByGameId(game models.Game) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	stmt := `UPDATE games SET title = $1, image = $2, release_date = $3 WHERE id = $4`
	result, err := m.DB.ExecContext(ctx, stmt, game.Title, game.Image, game.ReleaseDate, game.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return errors.New("Game was not found")
	}

	return nil
}

func (m *PostgresDBRepo) DeleteGameByGameId(id int) error {
	ctx, cancel := context.WithTimeout(context.Background(), dbTimeout)
	defer cancel()

	stmt := `DELETE FROM games WHERE id = $1`

	_, err := m.DB.ExecContext(ctx, stmt, id)
	if err != nil {
		return err
	}

	return nil
}