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

	// Root
	mux.Get("/", app.Home)

	// Auth
	mux.Post("/authenticate", app.Authenticate)
	mux.Get("/refresh-token", app.RefreshToken)
	mux.Get("/logout", app.Logout)

	// Games
	mux.Get("/games", app.AllGames)

	return mux
}