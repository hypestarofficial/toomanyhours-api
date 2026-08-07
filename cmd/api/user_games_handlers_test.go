package main

import (
	"net/http"
	"testing"

	"toomanyhours-api/internal/models"

	"gorm.io/gorm"
)

// twoUsers builds the fixture every ownership test needs: two accounts, each
// with one game in their list.
func twoUsers(t *testing.T, tx *gorm.DB) (mine, theirs *models.User, myGame, theirGame *models.Game) {
	t.Helper()
	mine = createUser(t, tx, "mine", "public")
	theirs = createUser(t, tx, "theirs", "public")
	myGame = createGame(t, tx, 1942, "The Witcher 3")
	theirGame = createGame(t, tx, 1020, "Grand Theft Auto V")
	addEntry(t, tx, mine.ID, myGame.ID, "finished")
	addEntry(t, tx, theirs.ID, theirGame.ID, "finished")
	return mine, theirs, myGame, theirGame
}

func TestGetMyGamesReturnsOnlyMine(t *testing.T) {
	app, tx := newTestApp(t)
	mine, _, myGame, _ := twoUsers(t, tx)

	w := do(t, app, http.MethodGet, "/me/games", nil, withAuth(accessToken(t, app, mine)))

	mustStatus(t, w, http.StatusOK)

	var entries []struct {
		GameID int `json:"gameId"`
	}
	decodeJSON(t, w, &entries)

	if len(entries) != 1 || entries[0].GameID != myGame.ID {
		t.Errorf("entries = %+v, want only game %d", entries, myGame.ID)
	}
}

func TestGetMyGamesEmptyListIsArrayNotNull(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "newbie", "public")

	w := do(t, app, http.MethodGet, "/me/games", nil, withAuth(accessToken(t, app, user)))

	mustStatus(t, w, http.StatusOK)

	// A nil slice marshals to null, and MyList maps over the result — a new
	// account would crash rather than show three empty sections.
	if got := w.Body.String(); got != "[]" {
		t.Errorf("body = %q, want %q", got, "[]")
	}
}

func TestPatchForeignEntryIs404(t *testing.T) {
	app, tx := newTestApp(t)
	mine, _, _, theirGame := twoUsers(t, tx)

	w := do(t, app, http.MethodPatch, path("/me/games/", theirGame.ID),
		map[string]any{"category": "want_to_play"}, withAuth(accessToken(t, app, mine)))

	// 404, not 403: a 403 would confirm that somebody has this game listed.
	mustStatus(t, w, http.StatusNotFound)
}

func TestPatchForeignEntryDoesNotChangeIt(t *testing.T) {
	app, tx := newTestApp(t)
	mine, theirs, _, theirGame := twoUsers(t, tx)

	do(t, app, http.MethodPatch, path("/me/games/", theirGame.ID),
		map[string]any{"category": "want_to_play"}, withAuth(accessToken(t, app, mine)))

	var entry models.UserGame
	if err := tx.Where("user_id = ? AND game_id = ?", theirs.ID, theirGame.ID).First(&entry).Error; err != nil {
		t.Fatalf("read their entry: %v", err)
	}
	if entry.Category != "finished" {
		t.Errorf("their category = %q, want %q — the 404 was cosmetic", entry.Category, "finished")
	}
}

func TestDeleteForeignEntryIs404(t *testing.T) {
	app, tx := newTestApp(t)
	mine, theirs, _, theirGame := twoUsers(t, tx)

	w := do(t, app, http.MethodDelete, path("/me/games/", theirGame.ID), nil, withAuth(accessToken(t, app, mine)))

	mustStatus(t, w, http.StatusNotFound)

	var count int64
	tx.Model(&models.UserGame{}).Where("user_id = ? AND game_id = ?", theirs.ID, theirGame.ID).Count(&count)
	if count != 1 {
		t.Errorf("their entry count = %d, want 1 — it was deleted", count)
	}
}

func TestPatchNonexistentEntryIs404(t *testing.T) {
	app, tx := newTestApp(t)
	mine, _, _, _ := twoUsers(t, tx)

	w := do(t, app, http.MethodPatch, "/me/games/99999999",
		map[string]any{"category": "want_to_play"}, withAuth(accessToken(t, app, mine)))

	mustStatus(t, w, http.StatusNotFound)
}

func TestDeleteNonexistentEntryIs404(t *testing.T) {
	app, tx := newTestApp(t)
	mine, _, _, _ := twoUsers(t, tx)

	w := do(t, app, http.MethodDelete, "/me/games/99999999", nil, withAuth(accessToken(t, app, mine)))

	mustStatus(t, w, http.StatusNotFound)
}

func TestPostDuplicateGameIs409(t *testing.T) {
	app, tx := newTestApp(t)
	mine, _, myGame, _ := twoUsers(t, tx)

	w := do(t, app, http.MethodPost, "/me/games",
		map[string]any{"igdbIds": []int{myGame.IGDBID}, "category": "want_to_play"},
		withAuth(accessToken(t, app, mine)))

	// The unique constraint is the authority, surfaced as
	// gorm.ErrDuplicatedKey by TranslateError. A pre-flight existence check
	// would have a race window.
	mustStatus(t, w, http.StatusConflict)
}

func TestPostBatchWithOneDuplicateRejectsAll(t *testing.T) {
	app, tx := newTestApp(t)
	mine, _, myGame, _ := twoUsers(t, tx)
	fresh := createGame(t, tx, 25076, "Red Dead Redemption 2")

	w := do(t, app, http.MethodPost, "/me/games",
		map[string]any{"igdbIds": []int{fresh.IGDBID, myGame.IGDBID}, "category": "want_to_play"},
		withAuth(accessToken(t, app, mine)))

	mustStatus(t, w, http.StatusConflict)

	// All or nothing is the contract, not an accident of one multi-row INSERT.
	var count int64
	tx.Model(&models.UserGame{}).Where("user_id = ? AND game_id = ?", mine.ID, fresh.ID).Count(&count)
	if count != 0 {
		t.Errorf("the non-duplicate game was added anyway (count = %d)", count)
	}
}

func TestPatchCanFinishAndRateInOneRequest(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")
	game := createGame(t, tx, 1942, "The Witcher 3")
	addEntry(t, tx, user.ID, game.ID, "currently_playing")

	w := do(t, app, http.MethodPatch, path("/me/games/", game.ID),
		map[string]any{"category": "finished", "rating": 8.5},
		withAuth(accessToken(t, app, user)))

	// The rule judges the *resulting* category. Checking the current one
	// breaks exactly this flow and nothing else.
	mustStatus(t, w, http.StatusOK)

	var entry models.UserGame
	if err := tx.Where("user_id = ? AND game_id = ?", user.ID, game.ID).First(&entry).Error; err != nil {
		t.Fatalf("read entry: %v", err)
	}
	if entry.Rating == nil || *entry.Rating != 8.5 {
		t.Errorf("rating = %v, want 8.5", entry.Rating)
	}
	if entry.Category != "finished" {
		t.Errorf("category = %q, want finished", entry.Category)
	}
}

func TestPatchRatingOnUnfinishedEntryIs400(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")
	game := createGame(t, tx, 1942, "The Witcher 3")
	addEntry(t, tx, user.ID, game.ID, "want_to_play")

	w := do(t, app, http.MethodPatch, path("/me/games/", game.ID),
		map[string]any{"rating": 8.5}, withAuth(accessToken(t, app, user)))

	mustStatus(t, w, http.StatusBadRequest)
}

func TestPatchOffStepRatingIs400(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")
	game := createGame(t, tx, 1942, "The Witcher 3")
	addEntry(t, tx, user.ID, game.ID, "finished")

	w := do(t, app, http.MethodPatch, path("/me/games/", game.ID),
		map[string]any{"rating": 6.25}, withAuth(accessToken(t, app, user)))

	// validate.Rating is the real defence. The column rounds to one decimal
	// before its CHECK runs, so the constraint would report a confusing 6.3.
	mustStatus(t, w, http.StatusBadRequest)
}

func TestMeGamesRequiresAToken(t *testing.T) {
	app, tx := newTestApp(t)
	_, _, myGame, _ := twoUsers(t, tx)

	cases := []struct {
		method string
		target string
		body   any
	}{
		{http.MethodGet, "/me/games", nil},
		{http.MethodPost, "/me/games", map[string]any{"igdbIds": []int{myGame.IGDBID}, "category": "want_to_play"}},
		{http.MethodPatch, path("/me/games/", myGame.ID), map[string]any{"category": "finished"}},
		{http.MethodDelete, path("/me/games/", myGame.ID), nil},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.target, func(t *testing.T) {
			w := do(t, app, tc.method, tc.target, tc.body)
			mustStatus(t, w, http.StatusUnauthorized)
		})
	}
}
