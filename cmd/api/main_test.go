package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"
	pg_driver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// testDB is the pool every test's transaction comes from. Nil when the
// database could not be reached, which makes every test skip rather than fail.
var testDB *gorm.DB

// dbUnavailable holds why there is no database, for the skip message.
var dbUnavailable string

// safeDBName is what may appear in a CREATE DATABASE statement. The name comes
// from the environment, and an identifier is not a parameter — it cannot be
// bound, only interpolated. Restricting the charset is what makes that safe.
var safeDBName = regexp.MustCompile(`^[a-z0-9_]+$`)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	// Tests run with cmd/api as the working directory.
	_ = godotenv.Load("../../.env")

	if err := setupTestDB(); err != nil {
		dbUnavailable = err.Error()
	}

	os.Exit(m.Run())
}

// setupTestDB creates the test database if needed, migrates it, and opens the
// shared pool. A returned error means "skip the suite"; anything that would be
// dangerous rather than merely unavailable calls log.Fatal instead.
func setupTestDB() error {
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	devName := os.Getenv("DB_NAME")

	if user == "" || host == "" || port == "" || devName == "" {
		return fmt.Errorf("DB_USER, DB_HOST, DB_PORT and DB_NAME must be set (looked in ../../.env)")
	}

	// Built by appending, never read from a variable that could name the dev
	// database. The check below is a backstop for that invariant, not the
	// mechanism enforcing it.
	testName := devName + "_test"
	if testName == devName || !strings.HasSuffix(testName, "_test") {
		log.Fatalf("refusing to run: test database name %q is not a _test database", testName)
	}
	if !safeDBName.MatchString(testName) {
		log.Fatalf("refusing to run: test database name %q is not a plain identifier", testName)
	}

	adminDSN := dsn(user, pass, host, port, "postgres")
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		return fmt.Errorf("open postgres at %s:%s: %w", host, port, err)
	}
	defer admin.Close()

	if err := admin.Ping(); err != nil {
		return fmt.Errorf("postgres unreachable at %s:%s (is `docker compose up -d` running?): %w", host, port, err)
	}

	var exists bool
	if err := admin.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, testName).Scan(&exists); err != nil {
		return fmt.Errorf("look up %s: %w", testName, err)
	}
	if !exists {
		if _, err := admin.Exec(fmt.Sprintf("CREATE DATABASE %q", testName)); err != nil {
			return fmt.Errorf("create %s: %w", testName, err)
		}
	}

	testDSN := dsn(user, pass, host, port, testName)

	// The real migrations, so the schema under test is the schema production
	// gets — including 000006's identity-sequence restart and every CHECK.
	mig, err := migrate.New("file://../../migrations", testDSN)
	if err != nil {
		return fmt.Errorf("open migrations: %w", err)
	}
	if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate %s: %w", testName, err)
	}
	if srcErr, dbErr := mig.Close(); srcErr != nil || dbErr != nil {
		return fmt.Errorf("close migrate: %v / %v", srcErr, dbErr)
	}

	sqlDB, err := sql.Open("pgx", testDSN)
	if err != nil {
		return fmt.Errorf("open %s: %w", testName, err)
	}
	gormDB, err := gorm.Open(pg_driver.New(pg_driver.Config{Conn: sqlDB}), &gorm.Config{
		// Without this a unique violation arrives as a raw pgconn error and
		// the 409 test would see a 500 — the same trap the production wiring
		// documents.
		TranslateError: true,
	})
	if err != nil {
		return fmt.Errorf("gorm open %s: %w", testName, err)
	}

	testDB = gormDB
	return nil
}

// dsn builds a URL-format connection string. url.UserPassword escapes a
// password containing characters that would otherwise break the URL.
func dsn(user, pass, host, port, name string) string {
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, pass),
		Host:     net.JoinHostPort(host, port),
		Path:     "/" + name,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}
