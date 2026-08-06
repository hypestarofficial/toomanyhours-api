package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"toomanyhours-api/internal/models"
	"toomanyhours-api/internal/validate"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// maxGamesPerRequest bounds a single POST. The modal caps selection at three,
// but the modal is not what is calling — curl is equally welcome. Without a cap
// the endpoint's cost is bounded only by the client's good manners.
const maxGamesPerRequest = 50

// currentUserID reads the id AuthRequired put in the context. Handlers must
// never take a user id from the body or the path: with /users/:id/games,
// forgetting one check anywhere lets anyone edit anyone's list by changing an
// integer. There is no id in these paths to forget.
func (app *application) currentUserID(c *gin.Context) (int, bool) {
	val, exists := c.Get("userID")
	if !exists {
		app.errorJSON(c, errors.New("User context missing"), http.StatusInternalServerError)
		return 0, false
	}

	userID, ok := val.(int)
	if !ok {
		app.errorJSON(c, errors.New("Invalid user ID type"), http.StatusInternalServerError)
		return 0, false
	}
	return userID, true
}

// gameIDParam parses :gameId from the path.
func (app *application) gameIDParam(c *gin.Context) (int, bool) {
	gameID, err := strconv.Atoi(c.Param("gameId"))
	if err != nil {
		app.errorJSON(c, errors.New("Invalid game id"), http.StatusBadRequest)
		return 0, false
	}
	return gameID, true
}

// GetMyGames returns the signed-in user's whole list.
func (app *application) GetMyGames(c *gin.Context) {
	userID, ok := app.currentUserID(c)
	if !ok {
		return
	}

	entries, err := app.DB.GetUserGames(c, userID)
	if err != nil {
		app.errorJSON(c, errors.New("Could not load your list"), http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, entries)
}

// PostMyGames adds games to the list, or moves them if already present.
func (app *application) PostMyGames(c *gin.Context) {
	userID, ok := app.currentUserID(c)
	if !ok {
		return
	}

	var requestPayload struct {
		GameIDs  []int  `json:"gameIds" binding:"required"`
		Category string `json:"category" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestPayload); err != nil {
		app.errorJSON(c, err, http.StatusBadRequest)
		return
	}

	if len(requestPayload.GameIDs) == 0 {
		app.errorJSON(c, errors.New("no games given"), http.StatusBadRequest)
		return
	}
	if len(requestPayload.GameIDs) > maxGamesPerRequest {
		app.errorJSON(c, fmt.Errorf("at most %d games per request", maxGamesPerRequest), http.StatusBadRequest)
		return
	}

	category, err := validate.Category(requestPayload.Category)
	if err != nil {
		app.errorJSON(c, fmt.Errorf("category: %w", err), http.StatusBadRequest)
		return
	}

	// Checked up front so an unknown id is a clear 404 rather than a
	// foreign-key violation surfacing as a 500.
	exists, err := app.DB.GamesExist(c, requestPayload.GameIDs)
	if err != nil {
		app.errorJSON(c, errors.New("Could not verify games"), http.StatusInternalServerError)
		return
	}
	if !exists {
		app.errorJSON(c, errors.New("Unknown game"), http.StatusNotFound)
		return
	}

	entries, err := app.DB.AddUserGames(c, userID, requestPayload.GameIDs, category)
	if err != nil {
		// Adding a game already in the list. Generic on purpose: naming which
		// one would need a second query on a path the picker makes rare, since
		// it no longer offers games you already have.
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			app.errorJSON(c, errors.New("One or more of those games is already in your list"), http.StatusConflict)
			return
		}
		app.errorJSON(c, errors.New("Could not add to your list"), http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusCreated, entries)
}

// PatchMyGame updates one entry's category, rating or review.
//
// Absent and null are indistinguishable to Go's JSON decoder, so clearing uses
// sentinel values the valid ranges exclude: rating 0 and review "".
func (app *application) PatchMyGame(c *gin.Context) {
	userID, ok := app.currentUserID(c)
	if !ok {
		return
	}
	gameID, ok := app.gameIDParam(c)
	if !ok {
		return
	}

	var requestPayload struct {
		Category *string `json:"category"`
		Rating   *int    `json:"rating"`
		Review   *string `json:"review"`
	}

	if err := c.ShouldBindJSON(&requestPayload); err != nil {
		app.errorJSON(c, err, http.StatusBadRequest)
		return
	}

	if requestPayload.Category == nil && requestPayload.Rating == nil && requestPayload.Review == nil {
		app.errorJSON(c, errors.New("nothing to update"), http.StatusBadRequest)
		return
	}

	var upd models.UserGameUpdate

	if requestPayload.Category != nil {
		category, err := validate.Category(*requestPayload.Category)
		if err != nil {
			app.errorJSON(c, fmt.Errorf("category: %w", err), http.StatusBadRequest)
			return
		}
		upd.Category = &category
	}

	if requestPayload.Rating != nil {
		upd.SetRating = true
		if *requestPayload.Rating != 0 {
			if err := validate.Rating(requestPayload.Rating); err != nil {
				app.errorJSON(c, fmt.Errorf("rating: %w", err), http.StatusBadRequest)
				return
			}
			upd.Rating = requestPayload.Rating
		}
		// A rating of 0 leaves upd.Rating nil with SetRating true: clear it.
	}

	if requestPayload.Review != nil {
		review, err := validate.Review(requestPayload.Review)
		if err != nil {
			app.errorJSON(c, fmt.Errorf("review: %w", err), http.StatusBadRequest)
			return
		}
		upd.SetReview = true
		// A blank review normalizes to nil: clear it.
		upd.Review = review
	}

	entry, err := app.DB.UpdateUserGame(c, userID, gameID, upd)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 404 rather than 403 even when the row exists under another user:
			// a 403 would confirm that somebody has this game listed.
			app.errorJSON(c, errors.New("Not in your list"), http.StatusNotFound)
			return
		}
		app.errorJSON(c, errors.New("Could not update your list"), http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, entry)
}

// DeleteMyGame removes one entry from the list.
func (app *application) DeleteMyGame(c *gin.Context) {
	userID, ok := app.currentUserID(c)
	if !ok {
		return
	}
	gameID, ok := app.gameIDParam(c)
	if !ok {
		return
	}

	if err := app.DB.DeleteUserGame(c, userID, gameID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			app.errorJSON(c, errors.New("Not in your list"), http.StatusNotFound)
			return
		}
		app.errorJSON(c, errors.New("Could not remove from your list"), http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusNoContent)
}
