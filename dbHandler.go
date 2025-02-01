package main

import (
	"log"

	"changeme/backend/inits"
	"changeme/backend/models"

	"github.com/wailsapp/wails/v3/pkg/application"
	"gorm.io/gorm"
)

type ShownVerse struct {
	models.Verse
	Book        models.Book
	Chapter     models.Chapter
	Translation models.Translation
}

type ShownCouplet struct {
	models.Couplet
	Song models.Song
}

type DbHandler struct {
	verse        *ShownVerse
	verseChannel chan *ShownVerse

	couplet        *ShownCouplet
	coupletChannel chan *ShownCouplet

	qr chan *bool

	app *application.App
}

func (g *DbHandler) showVerseInternal(verse *ShownVerse) {
	g.verse = verse
	g.verseChannel <- verse
	g.app.EmitEvent("show_verse", g.verse)
}

func (g *DbHandler) hideVerseInternal() {
	g.verse = nil
	g.verseChannel <- nil
	g.app.EmitEvent("hide_verse", nil)
}

func (g *DbHandler) showCoupletInternal(couplet *ShownCouplet) {
	g.couplet = couplet
	g.coupletChannel <- couplet
	g.app.EmitEvent("show_couplet", g.couplet)
}

func (g *DbHandler) hideCoupletInternal() {
	g.couplet = nil
	g.coupletChannel <- nil
	g.app.EmitEvent("hide_couplet", nil)
}

func addAscByNumber(db *gorm.DB) *gorm.DB {
	return db.Order("couplets.number ASC")
}

func (g *DbHandler) GetTranslations() []models.Translation {
	translations := []models.Translation{}
	err := inits.DB.Find(&translations).Error
	if err != nil {
		log.Println("Error fetching translations", err.Error())
		return nil
	}

	if len(translations) > 0 {
		firstTranslationBooks, err := g.getBooks(translations[0].ID)
		if err != nil {
			log.Println("Error fetching firstTranslationBooks", err.Error())
			return nil
		}
		translations[0].Books = firstTranslationBooks

		if len(translations[0].Books) > 0 {
			firstBookChapters, err := g.getChapters(translations[0].Books[0].ID)
			if err != nil {
				log.Println("Error fetching firstBookChapters", err.Error())
				return nil
			}
			translations[0].Books[0].Chapters = firstBookChapters

			if len(firstBookChapters) > 0 {
				firstChapterVerses, err := g.getVerses(firstBookChapters[0].ID)
				if err != nil {
					log.Println("Error fetching firstChapterVerses", err.Error())
					return nil
				}
				firstBookChapters[0].Verses = firstChapterVerses
			}
		}
	}

	return translations
}

func (g *DbHandler) getBooks(translationId uint) ([]models.Book, error) {
	books := []models.Book{}
	err := inits.DB.Order("number").Where("translation_id = ?", translationId).Find(&books).Error
	if err != nil {
		return nil, err
	}

	return books, nil
}

func (g *DbHandler) GetBooks(translationId float32) []models.Book {
	books, err := g.getBooks(uint(translationId))
	if err != nil {
		log.Println("Error fetching books", err.Error())
		return nil
	}

	if len(books) > 0 {
		firstBookChapters, err := g.getChapters(books[0].ID)
		if err != nil {
			log.Println("Error fetching firstBookChapters", err.Error())
			return nil
		}
		books[0].Chapters = firstBookChapters

		if len(firstBookChapters) > 0 {
			firstChapterVerses, err := g.getVerses(firstBookChapters[0].ID)
			if err != nil {
				log.Println("Error fetching firstChapterVerses", err.Error())
				return nil
			}
			firstBookChapters[0].Verses = firstChapterVerses
		}
	}

	return books
}

func (g *DbHandler) getChapters(bookId uint) ([]models.Chapter, error) {
	chapters := []models.Chapter{}
	err := inits.DB.Order("number").Where("book_id = ?", bookId).Find(&chapters).Error
	if err != nil {
		return nil, err
	}

	return chapters, nil
}

func (g *DbHandler) GetChapters(bookId float32) []models.Chapter {
	chapters, err := g.getChapters(uint(bookId))
	if err != nil {
		log.Println("Error fetching chapters", err.Error())
		return nil
	}

	if len(chapters) > 0 {
		firstChapterVerses, err := g.getVerses(chapters[0].ID)
		if err != nil {
			log.Println("Error fetching firstChapterVerses", err.Error())
			return nil
		}
		chapters[0].Verses = firstChapterVerses
	}

	return chapters
}

func (g *DbHandler) getVerses(chapterId uint) ([]models.Verse, error) {
	verses := []models.Verse{}
	err := inits.DB.Order("number").Where("chapter_id = ?", chapterId).Find(&verses).Error
	if err != nil {
		return nil, err
	}

	return verses, nil
}

func (g *DbHandler) GetVerses(chapterId float32) []models.Verse {
	verses, err := g.getVerses(uint(chapterId))
	if err != nil {
		log.Println("Error fetching verses", err.Error())
		return nil
	}

	return verses
}

func (g *DbHandler) ShowVerse(verseId float32) *ShownVerse {
	verseP := uint(verseId)

	verse := &models.Verse{}
	err := inits.DB.First(verse, verseP).Error
	if err != nil {
		log.Println("Error showing verse", err.Error())
		return nil
	}

	chapter := &models.Chapter{}
	err = inits.DB.First(chapter, verse.ChapterId).Error
	if err != nil {
		log.Println("Error showing chapter", err.Error())
		return nil
	}

	book := &models.Book{}
	err = inits.DB.First(book, chapter.BookId).Error
	if err != nil {
		log.Println("Error showing book", err.Error())
		return nil
	}

	translation := &models.Translation{}
	err = inits.DB.First(translation, book.TranslationId).Error
	if err != nil {
		log.Println("Error showing translation", err.Error())
		return nil
	}

	g.showVerseInternal(&ShownVerse{
		Verse:       *verse,
		Chapter:     *chapter,
		Book:        *book,
		Translation: *translation,
	})

	return g.verse
}

func (g *DbHandler) GetShownVerse() *ShownVerse {
	return g.verse
}

func (g *DbHandler) HideVerse() {
	g.hideVerseInternal()
}

func (g *DbHandler) GetSongs() []models.Song {
	songs := []models.Song{}
	err := inits.DB.Order("number").Find(&songs).Error
	if err != nil {
		log.Println("Error searching songs", err.Error())
		return nil
	}

	if len(songs) != 0 {
		couplets, err := g.getCouplets(songs[0].ID)
		if err != nil {
			log.Println("Error getting couplets for first song", err.Error())
			return nil
		}
		songs[0].Couplets = couplets
	}

	return songs
}

func (g *DbHandler) CreateSong(number int, title string) {
	dbSong := models.Song{Number: number, Title: title}
	err := inits.DB.Create(&dbSong).Error
	if err != nil {
		log.Println("Error creating song", err.Error())
		return
	}

	songs := g.GetSongs()
	if len(songs) > 0 {
		firstSongCouplets := g.GetCouplets(float32(songs[0].ID))
		songs[0].Couplets = firstSongCouplets
	}

	g.app.EmitEvent("songs_update", songs)
}

func (g *DbHandler) getCouplets(songId uint) ([]models.Couplet, error) {
	couplets := []models.Couplet{}
	err := inits.DB.Where("song_id = ?", songId).Order("number ASC").Find(&couplets).Error
	if err != nil {
		return nil, err
	}

	return couplets, nil
}

func (g *DbHandler) GetCouplets(songId float32) []models.Couplet {
	couplets, err := g.getCouplets(uint(songId))
	if err != nil {
		log.Println("Error getting couplets")
		return nil
	}

	return couplets
}

func (g *DbHandler) ShowCouplet(coupletFloatId float32) *ShownCouplet {
	coupletId := uint(coupletFloatId)

	couplet := &models.Couplet{}
	err := inits.DB.First(couplet, coupletId).Error
	if err != nil {
		log.Println("Error showing couplet", err.Error())
		return nil
	}

	song := &models.Song{}
	err = inits.DB.First(song, couplet.SongId).Error
	if err != nil {
		log.Println("Error showing song", err.Error())
		return nil
	}

	g.showCoupletInternal(&ShownCouplet{
		Couplet: *couplet,
		Song:    *song,
	})

	return g.couplet
}

func (g *DbHandler) CreateCouplet(text, label string, number, songId uint) {
	couplet := models.Couplet{
		Text:   text,
		Number: int(number),
		Label:  label,
		SongId: songId,
	}

	if err := inits.DB.Create(&couplet).Error; err != nil {
		log.Println("Error creating couplet", err.Error())
	}

	song := models.Song{}
	err := inits.DB.Preload("Couplets", addAscByNumber).Find(&song, songId).Error
	if err != nil {
		log.Println("Error getting new song state", err.Error())
		return
	}
	g.app.EmitEvent("song_update", song)
}

func (g *DbHandler) UpdateCouplet(coupletId int, label string, text string, number int) {
	couplet := &models.Couplet{}
	err := inits.DB.First(couplet, coupletId).Error
	if err != nil {
		log.Println("Error getting couplet", err.Error())
		return
	}

	couplet.Label = label
	couplet.Text = text
	couplet.Number = int(number)

	if err := inits.DB.Save(couplet).Error; err != nil {
		log.Println("Error updating couplet", err.Error())
		return
	}

	song := &models.Song{}
	err = inits.DB.Preload("Couplets", addAscByNumber).Find(song, couplet.SongId).Error
	if err != nil {
		log.Println("Error getting new song state", err.Error())
		return
	}

	g.app.EmitEvent("song_update", song)
}

func (g *DbHandler) RemoveCouplet(coupletId int) {
	couplet := models.Couplet{}
	err := inits.DB.Find(&couplet, coupletId).Error
	if err != nil {
		log.Println("Error deleting couplet", err.Error())
		return
	}

	if err := inits.DB.Delete(&models.Couplet{}, coupletId).Error; err != nil {
		log.Println("Error deleting couplet", err.Error())
		return
	}

	song := models.Song{}
	if err := inits.DB.Preload("Couplets", addAscByNumber).Find(&song, couplet.SongId).Error; err != nil {
		log.Println("Error getting new song state", err.Error())
		return
	}
	g.app.EmitEvent("song_update", song)
}

func (g *DbHandler) GetShownCouplet() *ShownCouplet {
	return g.couplet
}

func (g *DbHandler) HideCouplet() {
	g.hideCoupletInternal()
}

func (g *DbHandler) ShowQR() {
	r := true
	g.qr <- &r
}

func (g *DbHandler) HideQR() {
	r := false
	g.qr <- &r
}

func (g *DbHandler) ShowScreen(x, y, sizeX, sizeY float32, name string) {
	app := application.Get()

	app.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Name:        name,
		Title:       "VerseBearer - screen",
		Frameless:   true,
		X:           int(x),
		Y:           int(y),
		Width:       int(sizeX),
		Height:      int(sizeY),
		AlwaysOnTop: true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		// URL: "/screen",
		URL: "http://localhost:9093",
	})
}

func (g *DbHandler) CloseScreen(name string) {
	app := application.Get()

	s := app.GetWindowByName(name)
	s.Close()
}
