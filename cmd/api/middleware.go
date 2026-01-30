package main

import (
	"context"
	"fmt"
	"net/http"
)

type ContextKey string

const UserIDContextKey = ContextKey("userID")

func (app *application) enableCORS(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "")

		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, X-CSRF-Token, Authorization")
			return
		} else {
			h.ServeHTTP(w, r)
		}
	})
}

func (app *application) AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Use the updated helper function to get just the ID
		userID, err := app.auth.GetTokenFromHeaderAndVerify(w, r)
		if err != nil {
            // If verification fails, stop here and return unauthorized
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(fmt.Sprintf("Unauthorized: %s", err.Error()))) // Add error message for debugging
			return
		}
		
        // Add the user ID to the request context
        ctx := context.WithValue(r.Context(), UserIDContextKey, userID)
        
        // Call the next handler using the *new* request context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}