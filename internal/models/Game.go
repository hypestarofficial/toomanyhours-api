package models

import "time"

type Game struct {
	ID int `json:"id"`
	Title string `json:"title"`
	Image string `json:"image"`
	ReleaseDate time.Time `json:"releaseDate"`
}