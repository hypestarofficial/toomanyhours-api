package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
	"toomanyhours-api/internal/validate"

	"github.com/gin-gonic/gin"
)

// Reasons a name is unavailable. Distinct values because the form says
// different things: "reserved" is not "taken" to somebody staring at a name
// that looks free.
const (
	reasonTaken      = "taken"
	reasonReserved   = "reserved"
	reasonInvalid    = "invalid"
	reasonNotAllowed = "not_allowed"
)

type usernameAvailability struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

// CheckUsername reports whether a username could be registered.
//
// Advisory, and deliberately so: two people can claim a name between this
// answer and a submit, so the unique index stays the authority and registration
// still answers 409. A check that pretended to be a gate would be a second,
// weaker rule that disagrees with the database under load.
//
// Anonymous, because the register form has no token. It leaks nothing new:
// GET /profiles/:username already answers 403 for a taken private name and 404
// for a free one.
func (app *application) CheckUsername(c *gin.Context) {
	if retryAfter, blocked := app.usernameCheckBlocked(c.ClientIP()); blocked {
		// Round up: telling a client to wait 0 seconds invites an instant retry.
		seconds := int((retryAfter + time.Second - 1) / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		c.Header("Retry-After", strconv.Itoa(seconds))
		app.errorJSON(c, fmt.Errorf("Too many checks. Try again in %d seconds.", seconds), http.StatusTooManyRequests)
		return
	}

	// The same rules registration runs, not a copy. validate lowercases and
	// checks length, charset, reserved names and profanity in that order, so an
	// invalid name is answered without touching the database.
	name, err := validate.Username(c.Param("username"))
	if err != nil {
		switch {
		case errors.Is(err, validate.ErrReserved):
			c.JSON(http.StatusOK, usernameAvailability{Available: false, Reason: reasonReserved})
		case errors.Is(err, validate.ErrProfane):
			c.JSON(http.StatusOK, usernameAvailability{Available: false, Reason: reasonNotAllowed})
		default:
			c.JSON(http.StatusOK, usernameAvailability{Available: false, Reason: reasonInvalid})
		}
		return
	}

	_, err = app.DB.GetUserByUsername(c, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusOK, usernameAvailability{Available: true, Reason: ""})
			return
		}
		app.errorJSON(c, errors.New("Could not check that name"), http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, usernameAvailability{Available: false, Reason: reasonTaken})
}

// usernameCheckBlocked reports whether this address has spent its budget, and
// records the attempt when it has not.
func (app *application) usernameCheckBlocked(ip string) (time.Duration, bool) {
	ok, retryAfter := app.usernameCheckLimiter.Check(ip)
	if !ok {
		return retryAfter, true
	}
	app.usernameCheckLimiter.Record(ip)
	return 0, false
}
