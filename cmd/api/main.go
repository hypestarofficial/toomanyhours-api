package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
	"toomanyhours-api/internal/repository"
	"toomanyhours-api/internal/repository/dbrepo"
)

const port = 3130

type application struct {
	DSN string
	Domain string
	DB repository.DatabaseRepo
	auth Auth
	JWTSecret string
	JWTIssuer string
	JWTAudience string
	CookieDomain string
}

func main() {
	// set application config
	var app application

	// read from command line
	flag.StringVar(
		&app.DSN, 
		"dsn",
		"host=localhost port=5432 user=toomanyhours password=hypestar dbname=toomanyhours sslmode=disable timezone=UTC connect_timeout=5",
		"Postgres connection string",
	)
	flag.StringVar(
		&app.JWTSecret, "jwt-secret", "verysecret", "Signing secret" )
	flag.StringVar(
		&app.JWTIssuer, "jwt-issuer", "localhost", "Signing issuer" )
	flag.StringVar(
		&app.JWTAudience, "jwt-audience", "localhost", "Signing audience" )
	flag.StringVar(
		&app.CookieDomain, "cookie-domain", "localhost", "Cookie domain" )
	flag.StringVar(
		&app.Domain, "domain", "localhost", "Domain" )
	flag.Parse()

	// connect to database
	conn, err := app.connectToDB()
	if err != nil {
		log.Fatal(err)
	}
	app.DB = &dbrepo.PostgresDBRepo{DB: conn}
	defer app.DB.Connection().Close()

	app.auth = Auth{
		Issuer: app.JWTIssuer,
		Audience: app.JWTAudience,
		Secret: app.JWTSecret,
		TokenExpiry: time.Minute * 15,
		RefreshTokenExpiry: time.Hour * 24,
		CookiePath: "/", // root path
		CookieName: "__Host-refresh_token", // secure, httponly, samesite=lax cookie name
		CookieDomain: "",
	}

	log.Printf("starting API server on %s:%d", app.Domain, port)

	// start a web server
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), app.routes())
	if err != nil {
		log.Fatal(err)
	}
}