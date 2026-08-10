// Package database opens the one connection this project uses, the one way it
// uses it.
//
// It lives here rather than in cmd/api because a second binary needs it:
// cmd/backfill talks to the same database with the same settings, and a second
// copy of gorm.Open is a second place for TranslateError to be forgotten.
package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgconn"
	_ "github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	pg_driver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DSNFromEnv builds the key=value DSN from the DB_* variables.
//
// Deliberately not the URL-format MIGRATE_DATABASE_URL: golang-migrate cannot
// parse this form, which is why .env carries both.
func DSNFromEnv() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s timezone=%s connect_timeout=%s",
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"), os.Getenv("DB_SSLMODE"), os.Getenv("DB_TIMEZONE"), os.Getenv("DB_CONNECT_TIMEOUT"),
	)
}

// Open connects with pgx through database/sql and hands the connection to
// GORM.
func Open(dsn string) (*gorm.DB, error) {
	sqlDB, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, err
	}

	return gorm.Open(pg_driver.New(pg_driver.Config{Conn: sqlDB}), &gorm.Config{
		// Converts driver-specific errors (Postgres 23505 and friends) into
		// GORM sentinels such as gorm.ErrDuplicatedKey. Without this, a unique
		// violation arrives as a raw *pgconn.PgError, errors.Is against
		// gorm.ErrDuplicatedKey is always false, and handlers that mean to
		// return 409 silently return 500 instead.
		TranslateError: true,
	})
}
