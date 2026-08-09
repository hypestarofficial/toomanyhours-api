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

// PostMyGame adds one game to the list.
//
// Rating and review are optional and may only be written when the category is
// finished — the same rule PATCH applies, but simpler here. There is no
// current category to reconcile, so the posted category *is* the resulting
// one and validate.ResultingCategory is not involved.
func (app *application) PostMyGame(c *gin.Context) {
	userID, ok := app.currentUserID(c)
	if !ok {
		return
	}

	var requestPayload struct {
		// An IGDB id. A game the catalog has not seen is imported first. Local
		// catalog ids are not accepted: nothing browses the catalog now that
		// the picker searches IGDB.
		//
		// binding:"required" rejects 0, which is not a real IGDB id anyway.
		IGDBID   int      `json:"igdbId" binding:"required"`
		Category string   `json:"category" binding:"required"`
		Rating   *float64 `json:"rating"`
		Review   *string  `json:"review"`
	}

	if err := c.ShouldBindJSON(&requestPayload); err != nil {
		app.errorJSON(c, err, http.StatusBadRequest)
		return
	}

	category, err := validate.Category(requestPayload.Category)
	if err != nil {
		app.errorJSON(c, fmt.Errorf("category: %w", err), http.StatusBadRequest)
		return
	}

	// Only when the request actually carries one. An add with neither is the
	// ordinary case and skips this entirely.
	if requestPayload.Rating != nil || requestPayload.Review != nil {
		if !validate.RatingAllowed(category) {
			app.errorJSON(c, errors.New("only a finished game can be rated or reviewed"), http.StatusBadRequest)
			return
		}
	}

	// Unlike PATCH, 0 is not a clear sentinel here — there is nothing to clear
	// on a row that does not exist yet. validate.Rating rejects it as out of
	// range, so a client sending 0 for "unrated" fails loudly instead of
	// storing something no control can produce.
	if err := validate.Rating(requestPayload.Rating); err != nil {
		app.errorJSON(c, fmt.Errorf("rating: %w", err), http.StatusBadRequest)
		return
	}

	review, err := validate.Review(requestPayload.Review)
	if err != nil {
		app.errorJSON(c, fmt.Errorf("review: %w", err), http.StatusBadRequest)
		return
	}

	// Fetches and stores the game if the catalog has not seen it, so the rest
	// of this handler deals only in local ids.
	//
	// No GamesExist check follows: the id came from GamesByIGDBIDs or
	// ImportGames, so asking the database to confirm what it just told us
	// would buy nothing. An id IGDB does not know is already a 404 from here.
	gameIDs, err := app.importIGDBGames(c, []int{requestPayload.IGDBID})
	if err != nil {
		return // importIGDBGames has already answered
	}

	entry, err := app.DB.AddUserGame(c, userID, gameIDs[0], category, requestPayload.Rating, review)
	if err != nil {
		// Adding a game already in the list. The picker shows owned games
		// disabled, so this is reachable only from a stale client.
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			app.errorJSON(c, errors.New("That game is already in your list"), http.StatusConflict)
			return
		}
		app.errorJSON(c, errors.New("Could not add to your list"), http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusCreated, entry)
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
		Category *string  `json:"category"`
		Rating   *float64 `json:"rating"`
		Review   *string  `json:"review"`
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

	// Only when the request actually carries a rating or a review. A
	// category-only PATCH is the drag path, by far the hottest, and it keeps
	// doing exactly the queries it did before this rule existed.
	if requestPayload.Rating != nil || requestPayload.Review != nil {
		current, err := app.DB.GetUserGameCategory(c, userID, gameID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				app.errorJSON(c, errors.New("Not in your list"), http.StatusNotFound)
				return
			}
			app.errorJSON(c, errors.New("Could not update your list"), http.StatusInternalServerError)
			return
		}

		// The resulting category, not the current one: finishing a game moves it
		// and rates it in the same request. upd.Category is the normalized value
		// validate.Category returned, so "  Finished  " is judged as the category
		// actually about to be written.
		if !validate.RatingAllowed(validate.ResultingCategory(current, upd.Category)) {
			app.errorJSON(c, errors.New("You can only rate and review games you have finished"), http.StatusBadRequest)
			return
		}
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
