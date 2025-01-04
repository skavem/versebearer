package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"

	"changeme/backend/inits"
	"changeme/backend/models"
)

type BibleFileJson []struct {
	DividerBefore string     `json:"dividerBefore,omitempty"`
	Name          string     `json:"name"`
	FullName      string     `json:"fullName"`
	Content       [][]string `json:"content"`
}

type SongsFileJson []struct {
	Name     string `json:"name"`
	Label    string `json:"label"`
	Couplets []struct {
		Label string `json:"label"`
		Text  string `json:"text"`
		Index uint   `json:"index"`
	}
}

func main() {

	BibleFile, err := os.Open("Bible.json")
	if err != nil {
		panic("failed to open Bible.json")
	}
	defer BibleFile.Close()

	dec := json.NewDecoder(BibleFile)
	var Bible BibleFileJson
	err = dec.Decode(&Bible)
	if err != nil {
		panic("failed to decode Bible.json")
	}

	tranlation := models.Translation{Name: "Синодальный", ShortName: "SND"}
	inits.DB.Create(&tranlation)

	for _, book := range Bible {
		dbBook := models.Book{Title: book.FullName, ShortName: book.Name, TranslationId: tranlation.ID, DividerBefore: &book.DividerBefore}
		inits.DB.Create(&dbBook)

		for number, chapter := range book.Content {
			dbChapter := models.Chapter{Number: number + 1, BookId: dbBook.ID}

			for number, verse := range chapter {
				dbVerse := models.Verse{Text: verse, Number: number + 1}

				dbChapter.Verses = append(dbChapter.Verses, dbVerse)
			}

			inits.DB.Create(&dbChapter)
		}

		fmt.Println(book.Name, " added to translation")
	}

	songsFile, err := os.Open("songs.json")
	if err != nil {
		log.Panic("Error opening songs", err.Error())
	}
	defer songsFile.Close()

	songs := SongsFileJson{}
	dec = json.NewDecoder(songsFile)
	err = dec.Decode(&songs)
	if err != nil {
		panic("failed to decode songs.json")
	}

	for _, song := range songs {
		number, err := strconv.Atoi(song.Label)
		if err != nil {
			log.Println("Couldn't convert", song.Label, "to number")
			continue
		}

		couplets := []models.Couplet{}
		for _, c := range song.Couplets {
			couplets = append(couplets, models.Couplet{
				Text:   c.Text,
				Number: int(c.Index),
				Label:  c.Label,
			})
		}

		dbSong := models.Song{Title: song.Name, Number: number, Couplets: couplets}
		inits.DB.Create(&dbSong)
		log.Println("Song", song.Label, "was added")
	}

	mainScreen := models.Screen{
		Title:  "main",
		Layout: "[]",
	}
	inits.DB.Create(&mainScreen)

	inits.DB.Create(&models.GlobalState{
		Version:       "0.0.1",
		VerseScreenId: mainScreen.ID,
		SongScreenId:  mainScreen.ID,
	})
}
