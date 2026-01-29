package main

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v4"
)

func (app *application) Home(w http.ResponseWriter, r *http.Request) {
	var payload = struct {
		Status string `json:"status"`
		Message string `json:"message"`
		Version string `json:"version"`
	}{
		Status: "active",
		Message: "tooManyHours API is running",
		Version: "0.1.0",
	}

	_ = app.writeJSON(w, http.StatusOK, payload)
}

func (app *application) GetGames(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")

	games, err := app.DB.GetGames(title)
	if err != nil {
			app.errorJSON(w, err)
			return
	}

	_ = app.writeJSON(w, http.StatusOK, games)
}

func (app *application) GetGameByGameId(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	gameId, err := strconv.Atoi(id)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	game, err := app.DB.GetGameByGameId(gameId)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	_ = app.writeJSON(w, http.StatusOK, game)
}

func (app *application) GetGenres(w http.ResponseWriter, r *http.Request) {
	genres, err := app.DB.GetGenres()
	if err != nil {
			app.errorJSON(w, err)
			return
	}

	_ = app.writeJSON(w, http.StatusOK, genres)
}

func (app *application) Authenticate(w http.ResponseWriter, r *http.Request) {
	// read json payload
	var requestPayload struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}

	err := app.readJSON(w, r, &requestPayload)
	if err != nil {
		app.errorJSON(w, err, http.StatusBadRequest)
		return
	}

	// validate user against database
	user, err := app.DB.GetUserByEmail(requestPayload.Email)
	if err != nil {
		app.errorJSON(w, errors.New("Invalid credentials"), http.StatusBadRequest)
		return
	}

	// check credentials
	valid, err := user.PasswordMatches(requestPayload.Password)
	if err != nil || !valid {
		app.errorJSON(w, errors.New("Invalid credentials"), http.StatusBadRequest)
		return
	}

	// create a jwt user
	u := jwtUser{
		ID: user.ID,
		Username: user.Username,
	}

	// generate tokens
	tokens, err := app.auth.GenerateTokenPair(&u)
	if err != nil {
		app.errorJSON(w, err)
		return
	}

	refreshCookie := app.auth.GetRefreshCookie(tokens.RefreshToken)
	http.SetCookie(w, refreshCookie)

	app.writeJSON(w, http.StatusAccepted, tokens)
}

func (app *application) RefreshToken(w http.ResponseWriter, r *http.Request) {
	for _, cookie := range r.Cookies() {
		if cookie.Name == app.auth.CookieName {
			claims := &Claims{}
			refreshToken := cookie.Value

			// parse the token to get the claims
			_, err := jwt.ParseWithClaims(refreshToken, claims, func(token *jwt.Token) (any, error) {
				return []byte(app.auth.Secret), nil
			})

			if err != nil {
				app.errorJSON(w, errors.New(("Unauthorized")), http.StatusUnauthorized)
				return
			}

			// get the user id from the token claims
			userID, err := strconv.Atoi(claims.Subject)
			if err != nil {
				app.errorJSON(w, errors.New("Unknown user"), http.StatusUnauthorized)
				return
			}

			user, err := app.DB.GetUserByID(userID)
			if err != nil {
				app.errorJSON(w, errors.New("Unknown user"), http.StatusUnauthorized)
				return
			}

			u := jwtUser{
				ID: user.ID,
				Username: user.Username,
			}

			tokenPairs, err := app.auth.GenerateTokenPair(&u)
			if err != nil {
				app.errorJSON(w, errors.New("Error generating token pair"), http.StatusUnauthorized)
				return
			}

			http.SetCookie(w, app.auth.GetRefreshCookie(tokenPairs.RefreshToken))
			app.writeJSON(w, http.StatusOK, tokenPairs)
		}
	}
}

func (app *application) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, app.auth.GetExpiredRefreshCookie(""))
	w.WriteHeader(http.StatusAccepted)
}

func (app *application) GamesCatalog(w http.ResponseWriter, r *http.Request) {
	title := r.URL.Query().Get("title")

	games, err := app.DB.GetGames(title)
	if err != nil {
			app.errorJSON(w, err)
			return
	}

	_ = app.writeJSON(w, http.StatusOK, games)
}