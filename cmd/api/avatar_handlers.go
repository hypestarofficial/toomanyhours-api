package main

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"toomanyhours-api/internal/images"
	"toomanyhours-api/internal/validate"

	"github.com/gin-gonic/gin"
)

// maxAvatarUpload bounds the request body. The browser resizes to about 20KB
// before uploading; the headroom is for a direct upload. This is the first of
// two limits — it cannot stop a decode bomb, which is what images.Avatar's
// DecodeConfig check is for: a 60000-square PNG is forty bytes of header.
const maxAvatarUpload = 2 << 20 // 2 MB

// avatarDataURI is how an avatar reaches its owner: inline on GET /me.
//
// An <img> cannot send an Authorization header, and the public avatar route
// answers 403 for a private profile — including to its owner — so neither of
// the obvious alternatives works for the navbar.
func avatarDataURI(b []byte) string {
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(b)
}

// avatarFor returns a user's avatar as a data URI, or nil. Having no photo is
// the normal case rather than an error, so a failed read is simply nil.
func (app *application) avatarFor(c *gin.Context, userID int) *string {
	avatar, err := app.DB.GetUserAvatar(c, userID)
	if err != nil {
		return nil
	}

	uri := avatarDataURI(avatar.Bytes)
	return &uri
}

// PutMyAvatar accepts an uploaded image and stores a 256x256 JPEG.
func (app *application) PutMyAvatar(c *gin.Context) {
	userID, ok := app.userID(c)
	if !ok {
		return
	}

	// Wrapped before anything reads the body. A limit applied after the read is
	// not a limit.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAvatarUpload)

	file, err := c.FormFile("avatar")
	if err != nil {
		app.errorJSON(c, errors.New("Expected an image in the avatar field, at most 2MB"), http.StatusBadRequest)
		return
	}

	opened, err := file.Open()
	if err != nil {
		app.errorJSON(c, errors.New("Could not read the uploaded file"), http.StatusBadRequest)
		return
	}
	defer opened.Close()

	// Neither the filename nor the Content-Type is consulted: both are claims a
	// client makes. Decoding and re-encoding is what settles whether this is an
	// image, and it strips EXIF — including GPS — on the way through.
	encoded, err := images.Avatar(opened)
	if err != nil {
		switch {
		case errors.Is(err, images.ErrNotAnImage):
			app.errorJSON(c, errors.New("That file is not a JPEG, PNG or GIF image"), http.StatusBadRequest)
		case errors.Is(err, validate.ErrRange):
			app.errorJSON(c, errors.New("That image is too large to process"), http.StatusBadRequest)
		default:
			app.errorJSON(c, errors.New("Could not process that image"), http.StatusInternalServerError)
		}
		return
	}

	if err := app.DB.SetUserAvatar(c, userID, encoded, images.Hash(encoded)); err != nil {
		app.errorJSON(c, errors.New("Could not save that image"), http.StatusInternalServerError)
		return
	}

	app.respondWithUser(c, userID)
}

// DeleteMyAvatar removes a user's photo.
func (app *application) DeleteMyAvatar(c *gin.Context) {
	userID, ok := app.userID(c)
	if !ok {
		return
	}

	if err := app.DB.DeleteUserAvatar(c, userID); err != nil {
		app.errorJSON(c, errors.New("Could not remove that image"), http.StatusInternalServerError)
		return
	}

	app.respondWithUser(c, userID)
}

// GetProfileAvatar serves a public profile's photo.
//
// Anonymous, like GET /profiles/:username itself, and 403 for a private profile
// for the same reason: a link that stopped working should say why. There is no
// "who is asking" branch here either, which is precisely why an owner's own
// avatar comes inline on GET /me rather than from this route.
func (app *application) GetProfileAvatar(c *gin.Context) {
	// Normalised, not validated — the same treatment GetProfile gives it.
	username := strings.ToLower(strings.TrimSpace(c.Param("username")))
	if username == "" || len(username) > 64 {
		app.errorJSON(c, errors.New("No such profile"), http.StatusNotFound)
		return
	}

	user, err := app.DB.GetUserByUsername(c, username)
	if err != nil {
		app.errorJSON(c, errors.New("No such profile"), http.StatusNotFound)
		return
	}
	if user.Visibility != "public" {
		app.errorJSON(c, errors.New("This profile is private"), http.StatusForbidden)
		return
	}

	avatar, err := app.DB.GetUserAvatar(c, user.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.errorJSON(c, errors.New("No photo"), http.StatusNotFound)
			return
		}
		app.errorJSON(c, errors.New("Could not load that photo"), http.StatusInternalServerError)
		return
	}

	// immutable only when the caller asked for the version it is getting.
	// Without that check a hand-typed URL with no ?v would be cached for a year
	// and pin an old photo on screen with no way to shift it.
	if c.Query("v") == avatar.Hash {
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		c.Header("Cache-Control", "no-cache")
	}
	c.Header("ETag", fmt.Sprintf("%q", avatar.Hash))

	c.Data(http.StatusOK, "image/jpeg", avatar.Bytes)
}
