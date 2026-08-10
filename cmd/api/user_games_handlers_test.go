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
		map[string]any{"igdbId": myGame.IGDBID, "category": "want_to_play"},
		withAuth(accessToken(t, app, mine)))

	// The unique constraint is the authority, surfaced as
	// gorm.ErrDuplicatedKey by TranslateError. A pre-flight existence check
	// would have a race window.
	mustStatus(t, w, http.StatusConflict)
}

func TestPostCreatesEntryWithNeitherRatingNorReview(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")
	game := createGame(t, tx, 1942, "The Witcher 3")

	w := do(t, app, http.MethodPost, "/me/games",
		map[string]any{"igdbId": game.IGDBID, "category": "want_to_play"},
		withAuth(accessToken(t, app, user)))

	mustStatus(t, w, http.StatusCreated)

	var entry models.UserGame
	if err := tx.Where("user_id = ? AND game_id = ?", user.ID, game.ID).First(&entry).Error; err != nil {
		t.Fatalf("read entry: %v", err)
	}
	if entry.Rating != nil || entry.Review != nil {
		t.Errorf("rating = %v, review = %v, want both nil", entry.Rating, entry.Review)
	}
}

func TestPostFinishedCarriesRatingAndReview(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")
	game := createGame(t, tx, 1942, "The Witcher 3")

	w := do(t, app, http.MethodPost, "/me/games",
		map[string]any{"igdbId": game.IGDBID, "category": "finished", "rating": 8.5, "review": "  Great  "},
		withAuth(accessToken(t, app, user)))

	mustStatus(t, w, http.StatusCreated)

	var entry models.UserGame
	if err := tx.Where("user_id = ? AND game_id = ?", user.ID, game.ID).First(&entry).Error; err != nil {
		t.Fatalf("read entry: %v", err)
	}
	if entry.Rating == nil || *entry.Rating != 8.5 {
		t.Errorf("rating = %v, want 8.5", entry.Rating)
	}
	// validate.Review trims, so "cleared" has one representation rather than
	// two that every query would have to remember to check for.
	if entry.Review == nil || *entry.Review != "Great" {
		t.Errorf("review = %v, want \"Great\"", entry.Review)
	}
}

// The response is one object, not an array. A consumer that kept indexing
// into [0] must fail loudly rather than read a character out of a string.
func TestPostReturnsOneObjectNotAnArray(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")
	game := createGame(t, tx, 1942, "The Witcher 3")

	w := do(t, app, http.MethodPost, "/me/games",
		map[string]any{"igdbId": game.IGDBID, "category": "finished"},
		withAuth(accessToken(t, app, user)))

	mustStatus(t, w, http.StatusCreated)

	var entry struct {
		GameID   int    `json:"gameId"`
		Category string `json:"category"`
	}
	decodeJSON(t, w, &entry)

	if entry.GameID != game.ID || entry.Category != "finished" {
		t.Errorf("entry = %+v, want game %d finished", entry, game.ID)
	}
}

func TestPostRatingOnWantToPlayIs400(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")
	game := createGame(t, tx, 1942, "The Witcher 3")

	w := do(t, app, http.MethodPost, "/me/games",
		map[string]any{"igdbId": game.IGDBID, "category": "want_to_play", "rating": 8.5},
		withAuth(accessToken(t, app, user)))

	mustStatus(t, w, http.StatusBadRequest)

	// Rejected means nothing was written, not written-then-complained-about.
	var count int64
	tx.Model(&models.UserGame{}).Where("user_id = ? AND game_id = ?", user.ID, game.ID).Count(&count)
	if count != 0 {
		t.Errorf("entry was created anyway (count = %d)", count)
	}
}

func TestPostReviewOnCurrentlyPlayingIs400(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")
	game := createGame(t, tx, 1942, "The Witcher 3")

	w := do(t, app, http.MethodPost, "/me/games",
		map[string]any{"igdbId": game.IGDBID, "category": "currently_playing", "review": "Fun"},
		withAuth(accessToken(t, app, user)))

	mustStatus(t, w, http.StatusBadRequest)
}

// 0 is PATCH's "clear my rating" sentinel. There is nothing to clear on a row
// that does not exist yet, so POST rejects it — which is what makes a frontend
// sending 0 for "unrated" fail here rather than silently store nonsense.
func TestPostZeroRatingIs400(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")
	game := createGame(t, tx, 1942, "The Witcher 3")

	w := do(t, app, http.MethodPost, "/me/games",
		map[string]any{"igdbId": game.IGDBID, "category": "finished", "rating": 0},
		withAuth(accessToken(t, app, user)))

	mustStatus(t, w, http.StatusBadRequest)
}

func TestPostOffStepRatingIs400(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")
	game := createGame(t, tx, 1942, "The Witcher 3")

	w := do(t, app, http.MethodPost, "/me/games",
		map[string]any{"igdbId": game.IGDBID, "category": "finished", "rating": 6.25},
		withAuth(accessToken(t, app, user)))

	// validate.Rating is the real defence. The column rounds to one decimal
	// before its CHECK runs, so the constraint would report a confusing 6.3.
	mustStatus(t, w, http.StatusBadRequest)
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

// The parent has to survive the round trip to the client — that column is what
// the frontend reads to decide whether to hide an entry, and a field the model
// does not serialise is invisible however well it stores.
//
// This does not exercise a real IGDB import: every posted igdbId must already
// be in the catalog, or the handler calls IGDB and the test app points at an
// unroutable address on purpose. The parse side is covered in internal/igdb.
func TestEntryResponseCarriesParentIgdbID(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")
	parent := createGame(t, tx, 903001, "Some Base Game")
	addon := createGame(t, tx, 903002, "Some Base Game: An Expansion")

	tx.Model(&models.Game{}).Where("id = ?", addon.ID).Update("parent_igdb_id", parent.IGDBID)

	w := do(t, app, http.MethodPost, "/me/games",
		map[string]any{"igdbId": addon.IGDBID, "category": "want_to_play"},
		withAuth(accessToken(t, app, user)))

	mustStatus(t, w, http.StatusCreated)

	var entry struct {
		Game struct {
			ParentIGDBID *int `json:"parentIgdbId"`
		} `json:"game"`
	}
	decodeJSON(t, w, &entry)

	if entry.Game.ParentIGDBID == nil || *entry.Game.ParentIGDBID != parent.IGDBID {
		t.Errorf("parentIgdbId = %v, want %d", entry.Game.ParentIGDBID, parent.IGDBID)
	}
}

// A column the model does not serialise is invisible however well it stores,
// and the frontend reads this off the embedded game.
func TestEntryResponseCarriesSummary(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "player", "public")
	game := createGame(t, tx, 903010, "Some Game")

	tx.Model(&models.Game{}).Where("id = ?", game.ID).Update("summary", "A short description.")

	w := do(t, app, http.MethodPost, "/me/games",
		map[string]any{"igdbId": game.IGDBID, "category": "want_to_play"},
		withAuth(accessToken(t, app, user)))

	mustStatus(t, w, http.StatusCreated)

	var entry struct {
		Game struct {
			Summary string `json:"summary"`
		} `json:"game"`
	}
	decodeJSON(t, w, &entry)

	if entry.Game.Summary != "A short description." {
		t.Errorf("summary = %q, want %q", entry.Game.Summary, "A short description.")
	}
}
