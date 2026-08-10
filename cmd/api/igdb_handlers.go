package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
	"toomanyhours-api/internal/igdb"
	"toomanyhours-api/internal/models"

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

// importIGDBGames turns IGDB ids into local game ids, fetching and storing any
// the catalog has not seen. It writes its own error response and returns a nil
// slice when something fails, so callers return immediately.
func (app *application) importIGDBGames(c *gin.Context, igdbIDs []int) ([]int, error) {
	if app.IGDB == nil {
		err := errors.New("Adding games from IGDB is not configured on this server")
		app.errorJSON(c, err, http.StatusServiceUnavailable)
		return nil, err
	}

	known, err := app.DB.GamesByIGDBIDs(c, igdbIDs)
	if err != nil {
		app.errorJSON(c, errors.New("Could not check the catalog"), http.StatusInternalServerError)
		return nil, err
	}

	missing := make([]int, 0, len(igdbIDs))
	for _, id := range igdbIDs {
		if _, ok := known[id]; !ok {
			missing = append(missing, id)
		}
	}

	if len(missing) > 0 {
		fetched, err := app.IGDB.GetByIDs(c, missing)
		if err != nil {
			log.Printf("igdb fetch %v: %v", missing, err)
			app.errorJSON(c, errors.New("Could not reach IGDB"), http.StatusBadGateway)
			return nil, err
		}
		if len(fetched) != len(missing) {
			// An id IGDB does not know is a 404, not a 502: the request named
			// something that does not exist.
			app.errorJSON(c, errors.New("Unknown game"), http.StatusNotFound)
			return nil, errors.New("igdb returned fewer games than requested")
		}

		toStore := make([]*models.Game, 0, len(fetched))
		for _, g := range fetched {
			toStore = append(toStore, igdbToGame(g))
		}

		stored, err := app.DB.ImportGames(c, toStore)
		if err != nil {
			log.Printf("import %v: %v", missing, err)
			app.errorJSON(c, errors.New("Could not add those games to the catalog"), http.StatusInternalServerError)
			return nil, err
		}
		for igdbID, localID := range stored {
			known[igdbID] = localID
		}
	}

	// Preserve the caller's order, so the response matches what was asked for.
	out := make([]int, 0, len(igdbIDs))
	for _, id := range igdbIDs {
		out = append(out, known[id])
	}
	return out, nil
}

// igdbToGame converts a search result into a catalog row.
func igdbToGame(g igdb.Game) *models.Game {
	game := &models.Game{
		IGDBID:       g.IGDBID,
		Title:        g.Title,
		Kind:         g.Kind,
		ParentIGDBID: g.ParentIGDBID,
	}
	if g.Image != nil {
		game.Image = *g.Image
	}
	if g.ReleaseDate != nil {
		// Parsed back from the YYYY-MM-DD the client formatted. Storing the
		// zero time for an unreleased game is fine: release_date is only ever
		// displayed.
		if t, err := time.Parse("2006-01-02", *g.ReleaseDate); err == nil {
			game.ReleaseDate = t
		}
	}

	for _, t := range g.Genres {
		game.Tags = append(game.Tags, &models.Tag{Facet: "genre", IGDBID: t.IGDBID, Name: t.Name})
	}
	for _, t := range g.Themes {
		game.Tags = append(game.Tags, &models.Tag{Facet: "theme", IGDBID: t.IGDBID, Name: t.Name})
	}
	for _, t := range g.GameModes {
		game.Tags = append(game.Tags, &models.Tag{Facet: "game_mode", IGDBID: t.IGDBID, Name: t.Name})
	}

	return game
}
