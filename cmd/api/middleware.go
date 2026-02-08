package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type ContextKey string

const UserIDContextKey = ContextKey("userID")

func (app *application) enableCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "https://*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, X-CSRF-Token, Authorization")
	
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func (app *application) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, err := app.auth.GetTokenFromHeaderAndVerify(c)
		if err != nil {
			app.errorJSON(c, err, http.StatusUnauthorized)
			return
		}
		
		c.Set("userID", userID)
		c.Next()
	}
}