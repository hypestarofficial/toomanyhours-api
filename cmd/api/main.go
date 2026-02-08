package main

import (
	"flag"
	"fmt"
	"log"
	"net/mail"
	"os"
	"time"
	"toomanyhours-api/internal/repository"
	"toomanyhours-api/internal/repository/dbrepo"

	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
)

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

func emailValidator(fl validator.FieldLevel) bool {
if _, err := mail.ParseAddress(fl.Field().String()); err == nil {
	return true
}
return false
}

func init() {
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
			v.RegisterValidation("email", emailValidator)
	}
}

func main() {
	// set application config
	var app application
	godotenv.Load()

	// get environment variables
	version := os.Getenv("VERSION")
	port := os.Getenv("PORT")
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	dbPort := os.Getenv("DB_PORT")
	dbSslmode := os.Getenv("DB_SSLMODE")
	dbTimezone := os.Getenv("DB_TIMEZONE")
	dbTimeout := os.Getenv("DB_CONNECT_TIMEOUT")
	dbHost := os.Getenv("DB_HOST")

	// read from command line
	flag.StringVar(
		&app.DSN, 
		"dsn",
		fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s timezone=%s connect_timeout=%s", dbHost, dbPort, dbUser, dbPass, dbName, dbSslmode, dbTimezone, dbTimeout),
		"Postgres connection string",
	)
	flag.StringVar(
		&app.JWTSecret, "jwt-secret", os.Getenv("JWT_SECRET"), "Signing secret" )
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

	log.Printf("starting tooManyHours API server on %s:%s", app.Domain, port)
	log.Printf("version: %s", version)

	// start a web server
	err = app.routes().Run(fmt.Sprintf(":%s", port)) 
	if err != nil {
		log.Fatal(err)
	}
}