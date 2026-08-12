package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestGetProfilePublicReturnsTheList(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	game := createGame(t, tx, 1942, "The Witcher 3")
	addEntry(t, tx, user.ID, game.ID, "finished")

	w := do(t, app, http.MethodGet, "/profiles/hype", nil)

	mustStatus(t, w, http.StatusOK)

	var body struct {
		Username  string `json:"username"`
		CreatedAt string `json:"createdAt"`
		Entries   []struct {
			GameID   int    `json:"gameId"`
			Category string `json:"category"`
		} `json:"entries"`
	}
	decodeJSON(t, w, &body)

	if body.Username != "hype" {
		t.Errorf("username = %q, want %q", body.Username, "hype")
	}
	if body.CreatedAt == "" {
		t.Error("createdAt is empty")
	}
	if len(body.Entries) != 1 || body.Entries[0].GameID != game.ID {
		t.Errorf("entries = %+v, want one entry for game %d", body.Entries, game.ID)
	}
}

func TestGetProfilePrivateIs403(t *testing.T) {
	app, tx := newTestApp(t)
	createUser(t, tx, "shy", "private")

	w := do(t, app, http.MethodGet, "/profiles/shy", nil)

	// 403 rather than 404 on purpose: a link that stopped working should say
	// why. Unlike /me/games, where a foreign entry answers 404 so the response
	// cannot confirm it exists.
	mustStatus(t, w, http.StatusForbidden)
}

func TestGetProfileUnknownIs404(t *testing.T) {
	app, _ := newTestApp(t)

	w := do(t, app, http.MethodGet, "/profiles/nobodyhere", nil)

	mustStatus(t, w, http.StatusNotFound)
}

func TestGetProfileIsCaseInsensitive(t *testing.T) {
	app, tx := newTestApp(t)
	createUser(t, tx, "hype", "public")

	w := do(t, app, http.MethodGet, "/profiles/HyPe", nil)

	// The lookup lowercases. It does not run validate.Username, which also
	// rejects reserved and profane names — those are rules about creating an
	// account, and `admin` is both reserved and a real row.
	mustStatus(t, w, http.StatusOK)
}

func TestGetProfileOverlongUsernameIs404(t *testing.T) {
	app, _ := newTestApp(t)

	w := do(t, app, http.MethodGet, "/profiles/"+strings.Repeat("a", 200), nil)

	mustStatus(t, w, http.StatusNotFound)
}

func TestGetProfileBlankUsernameIs404(t *testing.T) {
	app, _ := newTestApp(t)

	// Percent-encoded spaces rather than a bare trailing slash: Gin's
	// RedirectTrailingSlash would answer `/profiles/` with a 301 and never
	// reach the handler. This does reach it, and the handler's own guard —
	// which trims before checking for empty — is what answers.
	w := do(t, app, http.MethodGet, "/profiles/%20%20", nil)

	mustStatus(t, w, http.StatusNotFound)
}

func TestGetProfileNeverLeaksEmail(t *testing.T) {
	app, tx := newTestApp(t)
	createUser(t, tx, "hype", "public")

	w := do(t, app, http.MethodGet, "/profiles/hype", nil)

	mustStatus(t, w, http.StatusOK)

	// Asserted on raw bytes, not a decoded struct: the regression that killed
	// GET /users/:id was an extra field nobody was looking at.
	if strings.Contains(strings.ToLower(w.Body.String()), "email") {
		t.Errorf("response mentions email: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "@example.test") {
		t.Errorf("response contains an address: %s", w.Body.String())
	}
}

func TestGetProfileOwnersTokenDoesNotUnlockTheirPrivateProfile(t *testing.T) {
	app, tx := newTestApp(t)
	owner := createUser(t, tx, "shy", "private")

	w := do(t, app, http.MethodGet, "/profiles/shy", nil, withAuth(accessToken(t, app, owner)))

	// The route is always anonymous — there is no optional-auth middleware and
	// the handler has no "who is asking" branch. A private profile answers 403
	// to its owner too.
	mustStatus(t, w, http.StatusForbidden)
}

func TestGetProfileEntriesCarryGenres(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")
	game := createGame(t, tx, 1942, "The Witcher 3")
	// A high igdb_id on purpose. Migration 000006 seeds 20 real tags — its
	// tag inserts are unconditional, unlike its games_tags ones — so a
	// fixture using a real IGDB id collides on tags(facet, igdb_id).
	genre := createTag(t, tx, "genre", "Test Genre", 900012)
	linkTag(t, tx, game.ID, genre.ID)
	addEntry(t, tx, user.ID, game.ID, "finished")

	w := do(t, app, http.MethodGet, "/profiles/hype", nil)

	mustStatus(t, w, http.StatusOK)

	var body struct {
		Entries []struct {
			Game struct {
				Genres []struct {
					Name string `json:"name"`
				} `json:"genres"`
			} `json:"game"`
		} `json:"entries"`
	}
	decodeJSON(t, w, &body)

	if len(body.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(body.Entries))
	}
	// Proves SplitTags() ran after Preload("Game.Tags"). Forgetting it
	// serialises three empty arrays, which reads as a styling bug rather than
	// a loading one.
	if len(body.Entries[0].Game.Genres) != 1 || body.Entries[0].Game.Genres[0].Name != "Test Genre" {
		raw, _ := json.Marshal(body.Entries[0].Game)
		t.Errorf("genres missing from game: %s", raw)
	}
}

func TestGetProfileIncludesTheBio(t *testing.T) {
	app, tx := newTestApp(t)
	user := createUser(t, tx, "hype", "public")

	bio := "Plays too much Bethesda."
	if err := tx.Model(user).Update("bio", bio).Error; err != nil {
		t.Fatalf("set bio: %v", err)
	}

	w := do(t, app, http.MethodGet, "/profiles/hype", nil)
	mustStatus(t, w, http.StatusOK)

	var body struct {
		Bio *string `json:"bio"`
	}
	decodeJSON(t, w, &body)

	if body.Bio == nil || *body.Bio != bio {
		t.Errorf("bio = %v, want %q", body.Bio, bio)
	}
}

// A profile with no bio must send null rather than omit the key: the frontend
// types it as nullable, and an absent key reads as an older API rather than as
// an empty bio.
func TestGetProfileWithoutBioSendsNull(t *testing.T) {
	app, tx := newTestApp(t)
	createUser(t, tx, "quiet", "public")

	w := do(t, app, http.MethodGet, "/profiles/quiet", nil)
	mustStatus(t, w, http.StatusOK)

	if !strings.Contains(w.Body.String(), `"bio":null`) {
		t.Errorf("body = %s, want a null bio", w.Body.String())
	}
}
