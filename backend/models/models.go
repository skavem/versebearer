package models

import (
	"gorm.io/gorm"
)

type Translation struct {
	gorm.Model

	Books []Book `json:"books"`

	Name      string `json:"name"`
	ShortName string `json:"shortName"`
}

type Book struct {
	gorm.Model

	Title         string  `json:"title"`
	ShortName     string  `json:"shortName"`
	Number        int     `json:"number"`
	DividerBefore *string `json:"dividerBefore"`

	TranslationId uint
	Chapters      []Chapter `json:"chapters"`
}

type Chapter struct {
	gorm.Model

	Number int `json:"number"`

	BookId uint
	Verses []Verse `json:"verses"`
}

type Verse struct {
	gorm.Model

	Text   string `json:"text"`
	Number int    `json:"number"`

	ChapterId uint
}

type Song struct {
	gorm.Model

	Title  string `json:"title"`
	Number int    `json:"number"`

	Couplets []Couplet `json:"couplets"`
}

type Couplet struct {
	gorm.Model

	Text   string `json:"text"`
	Number int    `json:"number"`
	Label  string `json:"label"`

	SongId uint
}

type Screen struct {
	gorm.Model
	Title string `json:"title"`

	Layout string `json:"layout"`
}

type GlobalState struct {
	gorm.Model
	Version string `json:"version"`

	VerseScreenId uint
	VerseScreen   Screen

	SongScreenId uint
	SongScreen   Screen
}
