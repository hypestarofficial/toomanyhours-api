package main

import (
	"database/sql"
	"log"

	_ "github.com/jackc/pgconn"
	_ "github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/gorm"

	pg_driver "gorm.io/driver/postgres"
) 

func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}

	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (app *application) connectGormDB() (*gorm.DB, error) {
	sqlDB, err := openDB(app.DSN)
	if err != nil {
		return nil, err
	}

	gormDB, err := gorm.Open(pg_driver.New(pg_driver.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	log.Println("Connected to Postgres with GORM!")
	return gormDB, nil
}