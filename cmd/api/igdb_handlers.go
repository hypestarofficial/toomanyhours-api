package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"toomanyhours-api/internal/igdb"

	"github.com/gin-gonic/gin"
)

const (
	// One character matches most of the database and spends a request from a
	// budget of four per second.
	searchMinRunes = 2
	searchMaxRunes = 100

	searchDefaultLimit = 20
	searchMaxLimit     = 50
)

// SearchGames proxies a free-text search to IGDB.
//
// Results are pure IGDB: no database lookup, and the key is igdbId rather than
// id, because nothing here is a catalog row yet.
func (app *application) SearchGames(c *gin.Context) {
	if app.IGDB == nil {
		app.errorJSON(c, errors.New("Game search is not configured on this server"), http.StatusServiceUnavailable)
		return
	}

	// Runes, not bytes, for the same reason the review cap counts runes: a
	// query in Japanese must not hit the limit at a third of the length.
	query := strings.TrimSpace(c.Query("q"))
	if n := len([]rune(query)); n < searchMinRunes || n > searchMaxRunes {
		app.errorJSON(c, fmt.Errorf("q must be between %d and %d characters", searchMinRunes, searchMaxRunes), http.StatusBadRequest)
		return
	}

	limit := searchDefaultLimit
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			app.errorJSON(c, errors.New("limit must be a positive integer"), http.StatusBadRequest)
			return
		}
		limit = min(n, searchMaxLimit)
	}

	games, err := app.IGDB.Search(c, query, limit)
	if err != nil {
		if errors.Is(err, igdb.ErrNotConfigured) {
			app.errorJSON(c, errors.New("Game search is not configured on this server"), http.StatusServiceUnavailable)
			return
		}
		// Logged here, never forwarded: IGDB's error body is not ours to relay
		// and could echo the request, which carries our client id.
		log.Printf("igdb search %q: %v", query, err)
		app.errorJSON(c, errors.New("Game search is unavailable right now"), http.StatusBadGateway)
		return
	}

	c.JSON(http.StatusOK, games)
}
