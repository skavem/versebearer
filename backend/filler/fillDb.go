package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"changeme/backend/inits"
	"changeme/backend/models"
)

type BibleBook struct {
	DividerBefore string     `json:"dividerBefore,omitempty"`
	Name          string     `json:"name"`
	FullName      string     `json:"fullName"`
	Content       [][]string `json:"content"`
}

type BibleFileJson []BibleBook

// BibleFileWithHeader is the self-describing layout produced by the bible.by
// export: the translation name travels with the books instead of being
// hardcoded here. A file that is a bare array of books (the original
// Bible.json) still loads — see readBibleFile.
type BibleFileWithHeader struct {
	Name      string      `json:"name"`
	ShortName string      `json:"shortName"`
	Source    string      `json:"source"`
	Books     []BibleBook `json:"books"`
}

// readBibleFile accepts both layouts and always returns a named translation.
func readBibleFile(path string) (BibleFileWithHeader, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return BibleFileWithHeader{}, err
	}

	trimmed := strings.TrimLeft(string(raw), " \t\r\n\ufeff")
	if strings.HasPrefix(trimmed, "[") {
		var books BibleFileJson
		if err := json.Unmarshal(raw, &books); err != nil {
			return BibleFileWithHeader{}, err
		}
		return BibleFileWithHeader{Name: "Синодальный", ShortName: "SND", Books: books}, nil
	}

	var file BibleFileWithHeader
	if err := json.Unmarshal(raw, &file); err != nil {
		return BibleFileWithHeader{}, err
	}
	if file.Name == "" {
		file.Name = "Без названия"
	}
	return file, nil
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

	// Путь можно передать аргументом, чтобы залить любой выгруженный перевод:
	//   go run ./backend/filler tmp/bibles/nrt.json
	biblePath := "Bible.json"
	if len(os.Args) > 1 {
		biblePath = os.Args[1]
	}

	Bible, err := readBibleFile(biblePath)
	if err != nil {
		log.Panic("failed to read ", biblePath, ": ", err.Error())
	}

	tranlation := models.Translation{Name: Bible.Name, ShortName: Bible.ShortName}
	inits.DB.Create(&tranlation)
	log.Println("Translation", Bible.Name, "created from", biblePath)

	for number, book := range Bible.Books {
		divider := book.DividerBefore
		dbBook := models.Book{Title: book.FullName, ShortName: book.Name, Number: number + 1, TranslationId: tranlation.ID, DividerBefore: &divider}
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
	dec := json.NewDecoder(songsFile)
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
