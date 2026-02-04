package models

import "time"

type Game struct {
	ID int `json:"id"`
	Title string `json:"title"`
	Image string `json:"image"`
	ReleaseDate time.Time `json:"releaseDate"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
	Genres []*Genre `json:"genres,omitempty"`
	GenreIDs []int `json:"genreIds,omitempty"`
}

type Genre struct {
	ID int `json:"id"`
	Genre string `json:"genre"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}