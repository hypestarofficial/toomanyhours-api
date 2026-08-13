package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"
	"toomanyhours-api/internal/models"

	"github.com/gin-gonic/gin"
)

// maxUsernameLookup bounds what a public path segment can send to the database.
// The column's own limit is smaller; this is about not querying at all for
// something that cannot be a username.
const maxUsernameLookup = 64

// publicProfile is what a stranger sees.
//
// A named type rather than a gin.H, specifically so that adding a field is a
// deliberate act. The obvious mistake here is reaching for models.APIUser,
// which carries the email.
type publicProfile struct {
	Username string  `json:"username"`
	Bio      *string `json:"bio"`
	// The hash, not the image: the page builds ".../avatar?v=<hash>" from it,
	// which is what makes the photo cacheable for a year and still change the
	// moment it is replaced.
	AvatarHash *string            `json:"avatarHash"`
	CreatedAt  time.Time          `json:"createdAt"`
	Entries    []*models.UserGame `json:"entries"`
}

// GetProfile serves a public profile: whose list it is, and the list.
//
// The only route in this API reachable without a token. It is always anonymous
// — there is no optional auth — so a private profile answers 403 to its owner
// too. That keeps the handler to one code path with no "who is asking" branch,
// and 403 is what a visitor genuinely sees.
func (app *application) GetProfile(c *gin.Context) {
	// Normalised, not validated. validate.Username also rejects reserved and
	// profane names, which are rules about creating an account, not reading
	// one — and `admin` is both reserved and a real account here. A row that
	// exists must be readable.
	username := strings.ToLower(strings.TrimSpace(c.Param("username")))
	if username == "" || len(username) > maxUsernameLookup {
		app.errorJSON(c, errors.New("No such profile"), http.StatusNotFound)
		return
	}

	user, err := app.DB.GetUserByUsername(c, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.errorJSON(c, errors.New("No such profile"), http.StatusNotFound)
			return
		}
		app.errorJSON(c, errors.New("Could not load that profile"), http.StatusInternalServerError)
		return
	}

	// 403 rather than 404, deliberately unlike /me/games where a foreign entry
	// answers 404 so the response cannot confirm it exists. Here the friendlier
	// answer wins: a link that used to work should say why it stopped. The cost
	// is accepted — the status code confirms the username is taken.
	if user.Visibility != "public" {
		app.errorJSON(c, errors.New("This profile is private"), http.StatusForbidden)
		return
	}

	entries, err := app.DB.GetUserGames(c, user.ID)
	if err != nil {
		app.errorJSON(c, errors.New("Could not load that profile"), http.StatusInternalServerError)
		return
	}

	var avatarHash *string
	if avatar, err := app.DB.GetUserAvatar(c, user.ID); err == nil {
		avatarHash = &avatar.Hash
	}

	c.JSON(http.StatusOK, publicProfile{
		Username:   user.Username,
		Bio:        user.Bio,
		AvatarHash: avatarHash,
		CreatedAt:  user.CreatedAt,
		Entries:    entries,
	})
}
