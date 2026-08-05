package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"toomanyhours-api/internal/models"
	"toomanyhours-api/internal/validate"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// bcryptCost is deliberately above bcrypt.DefaultCost (10). The seeded fixture
// user was hashed at a different cost and still verifies, because bcrypt
// encodes the cost inside the hash itself.
const bcryptCost = 12

// dummyHash is compared against when a login references an unknown email, so
// that an unknown address costs the same time as a known one with the wrong
// password. Without it, response timing reveals which accounts exist.
var dummyHash []byte

func init() {
	dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equalization-placeholder"), bcryptCost)
}

// splitAndTrim splits a string by delimiter and trims whitespace from each element
func splitAndTrim(s string, delimiter string) []string {
	parts := strings.Split(s, delimiter)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (app *application) Home(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "active",
		"message": "tooManyHours API is running",
		"version": os.Getenv("VERSION"),
	})
}

func (app *application) MeHandler(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		app.errorJSON(c, errors.New("User context missing"), http.StatusInternalServerError)
		return
	}
	
	userID, ok := val.(int)
	if !ok {
		app.errorJSON(c, errors.New("Invalid user ID type"), http.StatusInternalServerError)
		return
	}

	user, err := app.DB.GetUserByID(c, userID)
	if err != nil {
		app.errorJSON(c, err)
		return
	}

	apiUser := models.APIUser{
		ID:         user.ID,
		Username:   user.Username,
		Email:      user.Email,
		Visibility: user.Visibility,
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
	}

	c.JSON(http.StatusOK, apiUser) 
}

func (app *application) PatchMe(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		app.errorJSON(c, errors.New("User context missing"), http.StatusInternalServerError)
		return
	}
	userID, ok := val.(int)
	if !ok {
		app.errorJSON(c, errors.New("Invalid user ID type"), http.StatusInternalServerError)
		return
	}

	// Pointers distinguish "field absent" from "field explicitly set to empty".
	var requestPayload struct {
		Username   *string `json:"username"`
		Visibility *string `json:"visibility"`
	}

	if err := c.ShouldBindJSON(&requestPayload); err != nil {
		app.errorJSON(c, err, http.StatusBadRequest)
		return
	}

	if requestPayload.Username == nil && requestPayload.Visibility == nil {
		app.errorJSON(c, errors.New("nothing to update"), http.StatusBadRequest)
		return
	}

	user, err := app.DB.GetUserByID(c, userID)
	if err != nil {
		app.errorJSON(c, errors.New("Unknown user"), http.StatusNotFound)
		return
	}

	if requestPayload.Username != nil {
		// Same validator as registration, so the two cannot drift apart.
		username, err := validate.Username(*requestPayload.Username)
		if err != nil {
			app.errorJSON(c, fmt.Errorf("username: %w", err), http.StatusBadRequest)
			return
		}
		user.Username = username
	}

	if requestPayload.Visibility != nil {
		if *requestPayload.Visibility != "public" && *requestPayload.Visibility != "private" {
			app.errorJSON(c, errors.New("visibility must be public or private"), http.StatusBadRequest)
			return
		}
		user.Visibility = *requestPayload.Visibility
	}

	if err := app.DB.UpdateUser(c, user); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			app.errorJSON(c, errors.New("username or email already taken"), http.StatusConflict)
			return
		}
		app.errorJSON(c, errors.New("Could not update account"), http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, models.APIUser{
		ID:         user.ID,
		Username:   user.Username,
		Email:      user.Email,
		Visibility: user.Visibility,
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
	})
}

func (app *application) GetGames(c *gin.Context) {
	title := c.Query("title")
	genreIDsStr := c.Query("genreIds")

	var genreIDs []int
	if genreIDsStr != "" {
		// Split comma-separated genre IDs
		genreIDsStrSlice := splitAndTrim(genreIDsStr, ",")
		for _, idStr := range genreIDsStrSlice {
			id, err := strconv.Atoi(idStr)
			if err == nil {
				genreIDs = append(genreIDs, id)
			}
		}
	}

	games, err := app.DB.GetGames(c,title, genreIDs)
	if err != nil {
			app.errorJSON(c, err, http.StatusInternalServerError)
			return
	}

	c.JSON(http.StatusOK, games)
}

func (app *application) GetGameByGameId(c *gin.Context) {
	id := c.Param("id")

	gameId, err := strconv.Atoi(id)
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	game, err := app.DB.GetGameByGameId(c, gameId)
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, game)
}

func (app *application) GetGenres(c *gin.Context) {
	genres, err := app.DB.GetGenres(c)
	if err != nil {
			app.errorJSON(c, err, http.StatusInternalServerError)
			return
	}

	c.JSON(http.StatusOK, genres)
}

func (app *application) Register(c *gin.Context) {
	var requestPayload struct {
		Username string `json:"username" binding:"required"`
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestPayload); err != nil {
		app.errorJSON(c, err, http.StatusBadRequest)
		return
	}

	// Same validators the rename flow uses, so the two cannot drift apart.
	// Both return the normalized value, which is what gets persisted.
	username, err := validate.Username(requestPayload.Username)
	if err != nil {
		app.errorJSON(c, fmt.Errorf("username: %w", err), http.StatusBadRequest)
		return
	}

	email, err := validate.Email(requestPayload.Email)
	if err != nil {
		app.errorJSON(c, fmt.Errorf("email: %w", err), http.StatusBadRequest)
		return
	}

	if err := validate.Password(requestPayload.Password); err != nil {
		app.errorJSON(c, errors.New("password must be between 8 and 72 characters"), http.StatusBadRequest)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(requestPayload.Password), bcryptCost)
	if err != nil {
		app.errorJSON(c, errors.New("Could not create account"), http.StatusInternalServerError)
		return
	}

	user := models.User{
		Username:   username,
		Email:      email,
		Password:   string(hashed),
		Visibility: "public",
	}

	if err := app.DB.CreateUser(c, &user); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			// Deliberately does not say which field collided: naming the email
			// would let anyone test whether an address has an account here.
			app.errorJSON(c, errors.New("username or email already taken"), http.StatusConflict)
			return
		}
		app.errorJSON(c, errors.New("Could not create account"), http.StatusInternalServerError)
		return
	}

	tokens, err := app.auth.GenerateTokenPair(&jwtUser{ID: user.ID, Username: user.Username})
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	http.SetCookie(c.Writer, app.auth.GetRefreshCookie(tokens.RefreshToken))
	c.JSON(http.StatusCreated, tokens)
}

func (app *application) Authenticate(c *gin.Context) {
	var requestPayload struct {
		Email string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	err := c.ShouldBindJSON(&requestPayload)
	if err != nil {
		app.errorJSON(c, err, http.StatusBadRequest)
		return
	}

	user, err := app.DB.GetUserByEmail(c, requestPayload.Email)
	if err != nil {
		// Spend the same time hashing as a real comparison would. Returning
		// immediately here made an unknown email measurably faster than a known
		// one with a wrong password, which leaks which accounts exist.
		bcrypt.CompareHashAndPassword(dummyHash, []byte(requestPayload.Password))
		app.errorJSON(c, errors.New("Invalid credentials"), http.StatusBadRequest)
		return
	}

	valid, err := user.PasswordMatches(requestPayload.Password)
	if err != nil || !valid {
		app.errorJSON(c, errors.New("Invalid credentials"), http.StatusBadRequest)
		return
	}

	u := jwtUser{
		ID: user.ID,
		Username: user.Username,
	}

	tokens, err := app.auth.GenerateTokenPair(&u)
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	refreshCookie := app.auth.GetRefreshCookie(tokens.RefreshToken)
	http.SetCookie(c.Writer, refreshCookie)

	c.JSON(http.StatusAccepted, tokens)
}

func (app *application) RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie(app.auth.CookieName)
	if err != nil {
		app.errorJSON(c, errors.New("Refresh token not found"), http.StatusUnauthorized)
		return
	}

	claims := &Claims{}
	_, err = jwt.ParseWithClaims(refreshToken, claims, func(token *jwt.Token) (any, error) {
		return []byte(app.auth.Secret), nil
	})

	if err != nil {
		app.errorJSON(c, errors.New("Unauthorized"), http.StatusUnauthorized)
		return
	}

	userID, err := strconv.Atoi(claims.Subject)
	if err != nil {
		app.errorJSON(c, errors.New("Unknown user"), http.StatusUnauthorized)
		return
	}

	user, err := app.DB.GetUserByID(c, userID)
	if err != nil {
		app.errorJSON(c, errors.New("Unknown user"), http.StatusUnauthorized)
		return
	}

	u := jwtUser{
		ID:       user.ID,
		Username: user.Username,
	}

	tokenPairs, err := app.auth.GenerateTokenPair(&u)
	if err != nil {
		app.errorJSON(c, errors.New("Error generating token pair"), http.StatusInternalServerError)
		return
	}

	http.SetCookie(c.Writer, app.auth.GetRefreshCookie(tokenPairs.RefreshToken))

	c.JSON(http.StatusOK, tokenPairs)
}


func (app *application) GetUserByID(c *gin.Context) {
	id := c.Param("id")

	userId, err := strconv.Atoi(id)
	if err != nil {
		app.errorJSON(c, err, http.StatusBadRequest)
		return
	}

	user, err := app.DB.GetUserByID(c, userId)
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	type UserResponse struct {
		ID        int       `json:"id"`
		Username  string    `json:"username"`
		Email     string    `json:"email"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	resp := UserResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	c.JSON(http.StatusOK, resp)
}

func (app *application) Logout(c *gin.Context) {
	c.SetCookie("__Host-refresh_token", "", -1, "/", "", true, true)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

func (app *application) PostGameToGames(c *gin.Context) {
	var game models.Game

	if err := c.ShouldBindJSON(&game); err != nil {
		app.errorJSON(c, err, http.StatusBadRequest)
		return
	}

	game.CreatedAt = time.Now()
	game.UpdatedAt = time.Now()

	newID, err := app.DB.PostGameToGames(c, game)
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	err = app.DB.PostGameGenres(c, newID, game.GenreIDs)
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	savedGame, err := app.DB.GetGameByGameId(c,newID)
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusCreated, savedGame)
}

func (app *application) PutGameByGameId(c *gin.Context) {
	id := c.Param("id")

	gameId, err := strconv.Atoi(id)
	if err != nil {
		app.errorJSON(c, err, http.StatusBadRequest)
		return
	}
	
	var incomingGame models.Game
	if err := c.ShouldBindJSON(&incomingGame); err != nil {
		app.errorJSON(c, err, http.StatusBadRequest)
		return
	}

	existingGame, err := app.DB.GetGameByGameId(c, gameId)
	if err != nil {
		app.errorJSON(c, err, http.StatusNotFound)
		return
	}

	existingGame.Title = incomingGame.Title
	existingGame.ReleaseDate = incomingGame.ReleaseDate
	existingGame.Image = incomingGame.Image
	existingGame.UpdatedAt = time.Now()

	err = app.DB.PutGameByGameId(c, *existingGame)
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	err = app.DB.PostGameGenres(c, existingGame.ID, incomingGame.GenreIDs)
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	// Re-fetch the game to get updated genres
	updatedGame, err := app.DB.GetGameByGameId(c, gameId)
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, updatedGame)
}

func (app *application) DeleteGameByGameId(c *gin.Context) {
	id := c.Param("id")

	gameId, err := strconv.Atoi(id)
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	err = app.DB.DeleteGameByGameId(c, gameId)
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, nil)
}