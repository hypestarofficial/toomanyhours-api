package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"toomanyhours-api/internal/igdb"
	"toomanyhours-api/internal/models"
	"toomanyhours-api/internal/ratelimit"
	"toomanyhours-api/internal/repository/dbrepo"

	"github.com/golang-jwt/jwt/v4"
	"gorm.io/gorm"
)

const testJWTSecret = "test-secret-not-a-real-one"

// newTestApp returns an application whose repository is bound to a fresh
// transaction, and that transaction for direct fixture work. The transaction
// is rolled back when the test ends, so nothing a test writes outlives it.
func newTestApp(t *testing.T) (*application, *gorm.DB) {
	t.Helper()

	if testDB == nil {
		t.Skip("test database unavailable: " + dbUnavailable)
	}

	tx := testDB.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	t.Cleanup(func() { tx.Rollback() })

	app := &application{
		GormDB: tx,
		DB:     &dbrepo.PostgresDBRepo{GormDB: tx},
		Domain: "localhost",
		auth: Auth{
			Issuer:             "localhost",
			Audience:           "localhost",
			Secret:             testJWTSecret,
			TokenExpiry:        15 * time.Minute,
			RefreshTokenExpiry: 24 * time.Hour,
			CookiePath:         "/",
			CookieName:         "__Host-refresh_token",
			CookieDomain:       "",
		},
		JWTSecret:         testJWTSecret,
		RefreshGrace:      10 * time.Second,
		loginIPLimiter:    ratelimit.New(20, 15*time.Minute),
		loginEmailLimiter: ratelimit.New(5, time.Minute),
		// Every limiter the app constructs in main() needs one here too: the
		// handlers call Check on it unconditionally, and a nil *Limiter panics
		// into a 500 with an empty body, which reads like a database failure.
		usernameCheckLimiter: ratelimit.New(60, time.Minute),
		// Non-nil so POST /me/games gets past its configuration check. Every
		// igdbId these tests post is already in the catalog, so `missing` is
		// empty and no request is ever made — the unroutable address is there
		// to make a regression loud rather than slow.
		IGDB: igdb.New(igdb.Config{
			ClientID:     "test-client",
			ClientSecret: "test-secret",
			TokenURL:     "http://127.0.0.1:1/oauth2/token",
			APIURL:       "http://127.0.0.1:1",
		}),
	}

	return app, tx
}

type reqOpt func(*http.Request)

func withAuth(token string) reqOpt {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+token) }
}

func withCookie(name, value string) reqOpt {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: name, Value: value}) }
}

// do drives the real router, so AuthRequired, the timeout middleware and CORS
// all run. Nothing here stubs the middleware — if it stops setting userID,
// these tests fail, which is the point.
func do(t *testing.T, app *application, method, target string, body any, opts ...reqOpt) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, opt := range opts {
		opt(req)
	}

	w := httptest.NewRecorder()
	app.routes().ServeHTTP(w, req)
	return w
}

func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
}

func createUser(t *testing.T, tx *gorm.DB, username, visibility string) *models.User {
	t.Helper()
	u := &models.User{
		Username: username,
		Email:    username + "@example.test",
		// Never authenticated in these tests, so a literal is enough and skips
		// bcrypt's cost. The column is NOT NULL.
		Password:   "not-a-real-hash",
		Visibility: visibility,
	}
	if err := tx.Create(u).Error; err != nil {
		t.Fatalf("create user %q: %v", username, err)
	}
	return u
}

func createGame(t *testing.T, tx *gorm.DB, igdbID int, title string) *models.Game {
	t.Helper()
	g := &models.Game{
		IGDBID:      igdbID,
		Title:       title,
		Image:       "//images.igdb.com/igdb/image/upload/t_cover_big/test.jpg",
		Kind:        "main_game",
		ReleaseDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	// Omit the many2many association: Tags is nil here, and letting GORM
	// manage it would try to write games_tags rows that do not exist yet.
	if err := tx.Omit("Tags").Create(g).Error; err != nil {
		t.Fatalf("create game %q: %v", title, err)
	}
	return g
}

func createTag(t *testing.T, tx *gorm.DB, facet, name string, igdbID int) *models.Tag {
	t.Helper()
	tag := &models.Tag{Facet: facet, IGDBID: igdbID, Name: name}
	if err := tx.Create(tag).Error; err != nil {
		t.Fatalf("create tag %q: %v", name, err)
	}
	return tag
}

func linkTag(t *testing.T, tx *gorm.DB, gameID, tagID int) {
	t.Helper()
	if err := tx.Exec(`INSERT INTO games_tags (game_id, tag_id) VALUES (?, ?)`, gameID, tagID).Error; err != nil {
		t.Fatalf("link tag %d to game %d: %v", tagID, gameID, err)
	}
}

func addEntry(t *testing.T, tx *gorm.DB, userID, gameID int, category string) *models.UserGame {
	t.Helper()
	e := &models.UserGame{UserID: userID, GameID: gameID, Category: category}
	if err := tx.Omit("Game").Create(e).Error; err != nil {
		t.Fatalf("add entry user=%d game=%d: %v", userID, gameID, err)
	}
	return e
}

// accessToken mints a real signed access token, so the Authorization header in
// tests is the same thing a browser sends.
func accessToken(t *testing.T, app *application, u *models.User) string {
	t.Helper()
	pairs, err := app.auth.GenerateTokenPair(&jwtUser{ID: u.ID, Username: u.Username}, "unused-jti")
	if err != nil {
		t.Fatalf("generate token pair: %v", err)
	}
	return pairs.Token
}

// startSession writes a refresh_tokens row and returns the pair for it, which
// is exactly what login does.
func startSession(t *testing.T, app *application, u *models.User) TokenPairs {
	t.Helper()
	pairs, err := app.issueSession(context.Background(), u, "")
	if err != nil {
		t.Fatalf("issue session: %v", err)
	}
	return pairs
}

// refreshRow reads a refresh token row straight from the transaction, for
// asserting on state the API does not expose.
func refreshRow(t *testing.T, tx *gorm.DB, jti string) *models.RefreshToken {
	t.Helper()
	var row models.RefreshToken
	if err := tx.Where("jti = ?", jti).First(&row).Error; err != nil {
		t.Fatalf("read refresh row %s: %v", jti, err)
	}
	return &row
}

// jtiOf pulls the id out of a signed refresh token, so tests can find its row.
func jtiOf(t *testing.T, app *application, refreshToken string) string {
	t.Helper()
	claims := &Claims{}
	if _, err := jwt.ParseWithClaims(refreshToken, claims, func(token *jwt.Token) (any, error) {
		return []byte(app.auth.Secret), nil
	}); err != nil {
		t.Fatalf("parse refresh token: %v", err)
	}
	if claims.ID == "" {
		t.Fatalf("refresh token carries no jti")
	}
	return claims.ID
}

func mustStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status = %d, want %d (body: %s)", w.Code, want, w.Body.String())
	}
}

// path joins a route prefix and an integer id. strconv rather than fmt so the
// call sites read as routes rather than format strings.
func path(prefix string, id int) string {
	return prefix + strconv.Itoa(id)
}
