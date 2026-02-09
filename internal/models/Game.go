package models

import "time"

type Game struct {
	ID int `json:"id" gorm:"primaryKey"`
	Title string `json:"title"`
	Image string `json:"image"`
	ReleaseDate time.Time `json:"releaseDate"`
	Genres []*Genre `json:"genres,omitempty" gorm:"many2many:games_genres;"` // many2many relationship with Genre model
	GenreIDs []int `json:"genreIds,omitempty" gorm:"-"` // not stored in database
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

type Genre struct {
	ID int `json:"id" gorm:"primaryKey"`
	Genre string `json:"genre"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}