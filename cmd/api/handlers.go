package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"toomanyhours-api/internal/models"
	"toomanyhours-api/internal/refresh"
	"toomanyhours-api/internal/validate"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// bcryptCost is deliberately above bcrypt.DefaultCost (10). The seeded fixture
// user was hashed at a different cost and still verifies, because bcrypt
// encodes the cost inside the hash itself.
const bcryptCost = 12

// dummyHash is compared against when a login references an unknown email, so
// that an unknown address costs the same time as a known one with the wrong
// password. Without it, response timing reveals which accounts exist.
var dummyHash []byte

func init() {
	dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equalization-placeholder"), bcryptCost)
}

// splitAndTrim splits a string by delimiter and trims whitespace from each element
func splitAndTrim(s string, delimiter string) []string {
	parts := strings.Split(s, delimiter)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (app *application) Home(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "active",
		"message": "tooManyHours API is running",
		"version": apiVersion,
	})
}

// userID pulls the authenticated user out of the Gin context, answering 500 and
// reporting false when it is missing — which can only happen if a route was
// registered outside AuthRequired.
func (app *application) userID(c *gin.Context) (int, bool) {
	val, exists := c.Get("userID")
	if !exists {
		app.errorJSON(c, errors.New("User context missing"), http.StatusInternalServerError)
		return 0, false
	}

	id, ok := val.(int)
	if !ok {
		app.errorJSON(c, errors.New("Invalid user ID type"), http.StatusInternalServerError)
		return 0, false
	}

	return id, true
}

// respondWithUser reloads the account and returns it, so a write to one part of
// it answers with the whole current shape.
//
// **One place building models.APIUser, rather than three literals.** That list
// has already dropped a new field twice — bio in 1.6.0, and parent_igdb_id
// before it in ImportGames' equivalent — because each literal names its fields
// explicitly and a missing one is silently absent rather than a compile error.
// Anything added to APIUser belongs here and only here.
func (app *application) respondWithUser(c *gin.Context, userID int) {
	user, err := app.DB.GetUserByID(c, userID)
	if err != nil {
		app.errorJSON(c, errors.New("Unknown user"), http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, models.APIUser{
		ID:         user.ID,
		Username:   user.Username,
		Email:      user.Email,
		Visibility: user.Visibility,
		Bio:        user.Bio,
		Avatar:     app.avatarFor(c, user.ID),
		CreatedAt:  user.CreatedAt,
		UpdatedAt:  user.UpdatedAt,
	})
}

func (app *application) MeHandler(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		app.errorJSON(c, errors.New("User context missing"), http.StatusInternalServerError)
		return
	}

	userID, ok := val.(int)
	if !ok {
		app.errorJSON(c, errors.New("Invalid user ID type"), http.StatusInternalServerError)
		return
	}

	app.respondWithUser(c, userID)
}

func (app *application) PatchMe(c *gin.Context) {
	val, exists := c.Get("userID")
	if !exists {
		app.errorJSON(c, errors.New("User context missing"), http.StatusInternalServerError)
		return
	}
	userID, ok := val.(int)
	if !ok {
		app.errorJSON(c, errors.New("Invalid user ID type"), http.StatusInternalServerError)
		return
	}

	// Pointers distinguish "field absent" from "field explicitly set to empty".
	var requestPayload struct {
		Username   *string `json:"username"`
		Visibility *string `json:"visibility"`
		Bio        *string `json:"bio"`
	}

	if err := c.ShouldBindJSON(&requestPayload); err != nil {
		app.errorJSON(c, err, http.StatusBadRequest)
		return
	}

	// Bio belongs in this guard, not only in the branch below: the request that
	// carries just a bio is also the request that clears one, and without this
	// it fails as an empty patch.
	if requestPayload.Username == nil && requestPayload.Visibility == nil && requestPayload.Bio == nil {
		app.errorJSON(c, errors.New("nothing to update"), http.StatusBadRequest)
		return
	}

	user, err := app.DB.GetUserByID(c, userID)
	if err != nil {
		app.errorJSON(c, errors.New("Unknown user"), http.StatusNotFound)
		return
	}

	if requestPayload.Username != nil {
		// Same validator as registration, so the two cannot drift apart.
		username, err := validate.Username(*requestPayload.Username)
		if err != nil {
			app.errorJSON(c, fmt.Errorf("username: %w", err), http.StatusBadRequest)
			return
		}
		user.Username = username
	}

	if requestPayload.Visibility != nil {
		if *requestPayload.Visibility != "public" && *requestPayload.Visibility != "private" {
			app.errorJSON(c, errors.New("visibility must be public or private"), http.StatusBadRequest)
			return
		}
		user.Visibility = *requestPayload.Visibility
	}

	if requestPayload.Bio != nil {
		// Empty string is the clear sentinel, the same convention rating 0 and
		// review "" use on /me/games: Go's JSON decoder cannot tell an absent
		// key from an explicit null, so "" is how "clear this" is said.
		bio, err := validate.Bio(requestPayload.Bio)
		if err != nil {
			app.errorJSON(c, fmt.Errorf("bio: %w", err), http.StatusBadRequest)
			return
		}
		user.Bio = bio
	}

	if err := app.DB.UpdateUser(c, user); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			app.errorJSON(c, errors.New("username or email already taken"), http.StatusConflict)
			return
		}
		app.errorJSON(c, errors.New("Could not update account"), http.StatusInternalServerError)
		return
	}

	// Through respondWithUser, not a second literal: this response is what the
	// frontend writes into its auth store, so a field missing here disappears
	// from the navbar on an unrelated action. Bio was nearly lost that way.
	app.respondWithUser(c, userID)
}

func (app *application) Register(c *gin.Context) {
	var requestPayload struct {
		Username string `json:"username" binding:"required"`
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestPayload); err != nil {
		app.errorJSON(c, err, http.StatusBadRequest)
		return
	}

	// Same validators the rename flow uses, so the two cannot drift apart.
	// Both return the normalized value, which is what gets persisted.
	username, err := validate.Username(requestPayload.Username)
	if err != nil {
		app.errorJSON(c, fmt.Errorf("username: %w", err), http.StatusBadRequest)
		return
	}

	email, err := validate.Email(requestPayload.Email)
	if err != nil {
		app.errorJSON(c, fmt.Errorf("email: %w", err), http.StatusBadRequest)
		return
	}

	if err := validate.Password(requestPayload.Password); err != nil {
		app.errorJSON(c, errors.New("password must be between 8 and 72 characters"), http.StatusBadRequest)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(requestPayload.Password), bcryptCost)
	if err != nil {
		app.errorJSON(c, errors.New("Could not create account"), http.StatusInternalServerError)
		return
	}

	user := models.User{
		Username:   username,
		Email:      email,
		Password:   string(hashed),
		Visibility: "public",
	}

	if err := app.DB.CreateUser(c, &user); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			// Deliberately does not say which field collided: naming the email
			// would let anyone test whether an address has an account here.
			app.errorJSON(c, errors.New("username or email already taken"), http.StatusConflict)
			return
		}
		app.errorJSON(c, errors.New("Could not create account"), http.StatusInternalServerError)
		return
	}

	tokens, err := app.issueSession(c, &user, "")
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	http.SetCookie(c.Writer, app.auth.GetRefreshCookie(tokens.RefreshToken))
	c.JSON(http.StatusCreated, tokens)
}

// loginRateKey normalizes an email for use as a limiter key, so Bob@x.com and
// bob@x.com share one bucket instead of doubling an attacker's budget.
//
// Deliberately not validate.Email: that returns an error for malformed input,
// and a login attempt with a junk address still needs counting rather than
// slipping onto a different code path.
func loginRateKey(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// loginBlocked reports whether either limiter refuses this attempt. It returns
// the longer of the two waits, so a client that honours Retry-After is not
// refused again the moment it retries.
func (app *application) loginBlocked(ipKey, emailKey string) (time.Duration, bool) {
	ipOK, ipWait := app.loginIPLimiter.Check(ipKey)
	emailOK, emailWait := app.loginEmailLimiter.Check(emailKey)

	if ipOK && emailOK {
		return 0, false
	}
	return max(ipWait, emailWait), true
}

// recordLoginFailure counts a failed attempt against both keys. Unknown email
// and wrong password count identically — distinguishing them here would leak
// which accounts exist, which the dummy-hash comparison exists to prevent.
func (app *application) recordLoginFailure(ipKey, emailKey string) {
	app.loginIPLimiter.RecordFailure(ipKey)
	app.loginEmailLimiter.RecordFailure(emailKey)
}

func (app *application) Authenticate(c *gin.Context) {
	var requestPayload struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	err := c.ShouldBindJSON(&requestPayload)
	if err != nil {
		app.errorJSON(c, err, http.StatusBadRequest)
		return
	}

	ipKey := c.ClientIP()
	emailKey := loginRateKey(requestPayload.Email)

	// Checked before the lookup and before bcrypt. Skipping that work is the
	// point: login is the only endpoint that hashes unauthenticated input, so
	// a check placed after it would stop guessing but not the CPU cost.
	if retryAfter, blocked := app.loginBlocked(ipKey, emailKey); blocked {
		// Round up: telling a client to wait 0 seconds invites an instant retry.
		seconds := int((retryAfter + time.Second - 1) / time.Second)
		if seconds < 1 {
			seconds = 1
		}

		c.Header("Retry-After", strconv.Itoa(seconds))
		app.errorJSON(
			c,
			fmt.Errorf("Too many login attempts. Try again in %d seconds.", seconds),
			http.StatusTooManyRequests,
		)
		return
	}

	user, err := app.DB.GetUserByEmail(c, requestPayload.Email)
	if err != nil {
		// Spend the same time hashing as a real comparison would. Returning
		// immediately here made an unknown email measurably faster than a known
		// one with a wrong password, which leaks which accounts exist.
		bcrypt.CompareHashAndPassword(dummyHash, []byte(requestPayload.Password))
		app.recordLoginFailure(ipKey, emailKey)
		app.errorJSON(c, errors.New("Invalid credentials"), http.StatusBadRequest)
		return
	}

	valid, err := user.PasswordMatches(requestPayload.Password)
	if err != nil || !valid {
		app.recordLoginFailure(ipKey, emailKey)
		app.errorJSON(c, errors.New("Invalid credentials"), http.StatusBadRequest)
		return
	}

	// Credentials were right, so earlier fumbles stop counting. Done here rather
	// than after token generation: a 500 minting tokens is not the user's fault
	// and should not leave them closer to a block.
	app.loginIPLimiter.Reset(ipKey)
	app.loginEmailLimiter.Reset(emailKey)

	// Bounded, indexed housekeeping on a path that is already spending time on
	// bcrypt, so there is no scheduler or goroutine to own.
	_ = app.DB.DeleteExpiredRefreshTokens(c)

	tokens, err := app.issueSession(c, user, "")
	if err != nil {
		app.errorJSON(c, err, http.StatusInternalServerError)
		return
	}

	refreshCookie := app.auth.GetRefreshCookie(tokens.RefreshToken)
	http.SetCookie(c.Writer, refreshCookie)

	c.JSON(http.StatusAccepted, tokens)
}

// endRefreshSession returns 401 and expires the cookie, so a browser holding a
// dead token stops sending it.
//
// The message never distinguishes unknown from expired from reused: this
// endpoint is unauthenticated, and the caller should learn only that it failed.
func (app *application) endRefreshSession(c *gin.Context) {
	http.SetCookie(c.Writer, app.auth.GetExpiredRefreshCookie(""))
	app.errorJSON(c, errors.New("Unauthorized"), http.StatusUnauthorized)
}

func (app *application) RefreshToken(c *gin.Context) {
	refreshToken, err := c.Cookie(app.auth.CookieName)
	if err != nil {
		app.errorJSON(c, errors.New("Refresh token not found"), http.StatusUnauthorized)
		return
	}

	claims := &Claims{}
	_, err = jwt.ParseWithClaims(refreshToken, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(app.auth.Secret), nil
	})
	if err != nil {
		app.endRefreshSession(c)
		return
	}

	// Access tokens are issued without an ID. Requiring one is what stops an
	// access token being presented here and exchanged for a fresh pair.
	if claims.ID == "" {
		app.endRefreshSession(c)
		return
	}

	row, err := app.DB.GetRefreshToken(c, claims.ID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		app.errorJSON(c, errors.New("Could not refresh"), http.StatusInternalServerError)
		return
	}

	state := refresh.State{}
	if row != nil {
		// Only asked when the row is revoked, since that is the only case where
		// the answer changes anything — and it is what tells a rotated token
		// apart from a session that has ended.
		familyActive := false
		if row.RevokedAt != nil {
			familyActive, err = app.DB.FamilyHasActiveToken(c, row.FamilyID)
			if err != nil {
				app.errorJSON(c, errors.New("Could not refresh"), http.StatusInternalServerError)
				return
			}
		}

		state = refresh.State{
			Found:        true,
			ExpiresAt:    row.ExpiresAt,
			RevokedAt:    row.RevokedAt,
			FamilyActive: familyActive,
		}
	}

	switch refresh.Decide(state, time.Now(), app.RefreshGrace) {
	case refresh.ReuseDetected:
		// Two parties presented the same consumed token while the session is
		// still live, so a copy leaked. Ending the whole chain is the only safe
		// response — and it logs the real user out too, which is the point:
		// they need to notice.
		_ = app.DB.RevokeRefreshFamily(c, row.FamilyID)
		app.endRefreshSession(c)
		return
	case refresh.RejectUnknown, refresh.RejectExpired, refresh.RejectRevoked:
		app.endRefreshSession(c)
		return
	}

	// The user, from the row rather than the token's Subject: one source of
	// truth, and the row is the one that can be revoked.
	user, err := app.DB.GetUserByID(c, row.UserID)
	if err != nil {
		app.endRefreshSession(c)
		return
	}

	// A no-op on the grace path, where the row is already revoked — the
	// repository's `revoked_at IS NULL` guard is what makes that safe.
	if err := app.DB.RevokeRefreshToken(c, row.JTI); err != nil {
		app.errorJSON(c, errors.New("Could not refresh"), http.StatusInternalServerError)
		return
	}

	tokenPairs, err := app.issueSession(c, user, row.FamilyID)
	if err != nil {
		app.errorJSON(c, errors.New("Could not refresh"), http.StatusInternalServerError)
		return
	}

	http.SetCookie(c.Writer, app.auth.GetRefreshCookie(tokenPairs.RefreshToken))

	c.JSON(http.StatusOK, tokenPairs)
}

// Logout revokes the session server-side. Before this, it only cleared the
// cookie in the caller's browser and the refresh token stayed valid for its
// full 24 hours — so "log out" meant "please forget this", not "this no longer
// works".
//
// Best effort by design: it always returns 200, even with a missing or
// unparseable cookie. Logout must not be able to fail, and telling an
// unauthenticated caller that their token was invalid is information this
// endpoint has no business handing out.
func (app *application) Logout(c *gin.Context) {
	if cookie, err := c.Cookie(app.auth.CookieName); err == nil {
		claims := &Claims{}
		_, parseErr := jwt.ParseWithClaims(cookie, claims, func(token *jwt.Token) (any, error) {
			return []byte(app.auth.Secret), nil
		})

		if parseErr == nil && claims.ID != "" {
			// The whole family, not just this token: logging out ends the
			// session, and the session is the rotation chain.
			if row, err := app.DB.GetRefreshToken(c, claims.ID); err == nil && row != nil {
				_ = app.DB.RevokeRefreshFamily(c, row.FamilyID)
			}
		}
	}

	http.SetCookie(c.Writer, app.auth.GetExpiredRefreshCookie(""))
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}
