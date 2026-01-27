package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"toomanyhours-api/internal/repository"
	"toomanyhours-api/internal/repository/dbrepo"
)

const port = 3130

type application struct {
	DSN string
	Domain string
	DB repository.DatabaseRepo
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
	flag.Parse()

	// connect to database
	conn, err := app.connectToDB()
	if err != nil {
		log.Fatal(err)
	}
	app.DB = &dbrepo.PostgresDBRepo{DB: conn}
	defer app.DB.Connection().Close()

	app.Domain = "localhost"

	log.Printf("starting API server on %s:%d", app.Domain, port)

	// start a web server
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), app.routes())
	if err != nil {
		log.Fatal(err)
	}
}