package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Routes using handlers
func (app *application) routes() *gin.Engine {
	// create a new multiplexer router
	router := gin.Default()

	// Gin trusts every proxy by default, which means c.ClientIP() returns
	// whatever X-Forwarded-For says — attacker-controlled. The login limiter
	// would then file every guess under a different key and stop limiting
	// anything. Trusting only loopback means the header is honoured for the
	// Vite dev proxy and ignored for anyone connecting from outside.
	//
	// A production reverse proxy is NOT loopback. Its address must be added
	// here, or c.ClientIP() falls back to the proxy's own IP and every user
	// shares one bucket. Nothing fails loudly when that happens.
	if err := router.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		log.Fatal(err)
	}

	router.Use(gin.Recovery())
	router.Use(app.enableCORS())
	router.Use(app.timeoutMiddleware(3 * time.Second))

	// Root for Ping (health check)
	router.GET("/", app.Home)

	// Auth for Register, Login, Refresh Token, and Logout
	router.POST("/register", app.Register)
	router.POST("/authenticate", app.Authenticate)
	router.GET("/refresh-token", app.RefreshToken)
	router.GET("/logout", app.Logout)

	// Authorized Routes
	auth := router.Group("/")
	// Check Auth Header for valid token
	auth.Use(app.AuthRequired())
	{
		// Me
		auth.GET("/me", app.MeHandler)
		auth.PATCH("/me", app.PatchMe)

		// My list
		auth.GET("/me/games", app.GetMyGames)
		auth.POST("/me/games", app.PostMyGames)
		auth.PATCH("/me/games/:gameId", app.PatchMyGame)
		auth.DELETE("/me/games/:gameId", app.DeleteMyGame)

		// Users
		auth.GET("/users/:id", app.GetUserByID)

		// Games
		auth.GET("/games", app.GetGames) // query params: title, genreIds
		// Static segment beside the :id wildcard. Verified to register in
		// either order and to resolve ahead of /games/:id, so "search" is not
		// swallowed as an id.
		auth.GET("/games/search", app.SearchGames) // query params: q, limit
		auth.GET("/games/:id", app.GetGameByGameId)

		// Genres
		auth.GET("/genres", app.GetGenres)
	}

	// Admin Routes
	admin := auth.Group("/admin")
	{
		// Admin Games Catalog (games with genres)
		admin.GET("/games", app.GetGames) // query params: title, genreIds
		admin.POST("/games", app.PostGameToGames)
		admin.PUT("/games/:id", app.PutGameByGameId)
		admin.DELETE("/games/:id", app.DeleteGameByGameId)
	}

	return router
}
