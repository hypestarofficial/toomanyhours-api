package main

import (
	"net/http"
	
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/chi/v5"
)

// Routes using handlers
func (app *application) routes() http.Handler {
	// create a new multiplexer router
	mux := chi.NewRouter()

	mux.Use(middleware.Recoverer)
	mux.Use(app.enableCORS)

	// Root for Ping (health check)
	mux.Get("/", app.Home)

	// Auth for Login, Refresh Token, and Logout
	mux.Post("/authenticate", app.Authenticate)
	mux.Get("/refresh-token", app.RefreshToken)
	mux.Get("/logout", app.Logout)

	// Authorized Routes
	mux.Group(func(mux chi.Router) {
		// Check Auth Header for valid token
		mux.Use(app.AuthRequired)

		// Me
		mux.Get("/me", app.MeHandler)

		// Users
		mux.Get("/users/{id}", app.GetUserByID)
		
		// Games
		mux.Get("/games", app.GetGames) // query params: title
		mux.Get("/games/{id}", app.GetGameByGameId)

		// Genres
		mux.Get("/genres", app.GetGenres)
	})

	// Admin Routes
	mux.Route("/admin", func(mux chi.Router) {
		// Check Auth Header for valid token
		mux.Use(app.AuthRequired)
		
		// Admin Games Catalog (games with genres)
		mux.Get("/games", app.GamesCatalog)
	})

	return mux
}