package main

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"toomanyhours-api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
)

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

func (app *application) GetGames(c *gin.Context) {
	title := c.Query("title")
	genreIDsStr := c.Query("genreIDs")

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