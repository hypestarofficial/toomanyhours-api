package main

import (
	"net/http"
	
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/chi/v5"
)

func (app *application) routes() http.Handler {
	// create a new multiplexer router
	mux := chi.NewRouter()

	mux.Use(middleware.Recoverer)
	mux.Use(app.enableCORS)

	mux.Get("/", app.Home)
	mux.Get("/games", app.AllGames)

	// return the multiplexer
	return mux
}