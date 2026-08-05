package main

import (
	"time"

	"github.com/gin-gonic/gin"
)

// Routes using handlers
func (app *application) routes() *gin.Engine {
	// create a new multiplexer router
	router := gin.Default()

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

		// Users
		auth.GET("/users/:id", app.GetUserByID)
		
		// Games
		auth.GET("/games", app.GetGames) // query params: title, genreIDs
		auth.GET("/games/:id", app.GetGameByGameId)

		// Genres
		auth.GET("/genres", app.GetGenres)
	}

	// Admin Routes
	admin := auth.Group("/admin")
	{
		// Admin Games Catalog (games with genres)
		admin.GET("/games", app.GetGames) // query params: title, genreIDs
		admin.POST("/games", app.PostGameToGames)
		admin.PUT("/games/:id", app.PutGameByGameId)
		admin.DELETE("/games/:id", app.DeleteGameByGameId)
	}

	return router
}