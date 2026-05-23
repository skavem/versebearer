package main

import (
	"log"

	"changeme/backend/inits"
	"changeme/backend/models"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"gorm.io/gorm"
)

const mainWindowName = "VerseBearer"

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
	verseB   *broadcaster[ShownVerse]
	coupletB *broadcaster[ShownCouplet]

	qr chan *bool

	app *application.App
}

type broadcaster[T any] struct {
	state   *T
	ch      chan *T
	showEvt string
	hideEvt string
	emit    func(name string, data any)
}

func (b *broadcaster[T]) show(val *T) {
	b.state = val
	b.ch <- val
	b.emit(b.showEvt, val)
}

func (b *broadcaster[T]) hide() {
	b.state = nil
	b.ch <- nil
	b.emit(b.hideEvt, nil)
}

func (g *DbHandler) emit(name string, data any) {
	if g.app != nil {
		g.app.Event.Emit(name, data)
	}
}

func (g *DbHandler) showVerseInternal(verse *ShownVerse)       { g.verseB.show(verse) }
func (g *DbHandler) hideVerseInternal()                        { g.verseB.hide() }
func (g *DbHandler) showCoupletInternal(couplet *ShownCouplet) { g.coupletB.show(couplet) }
func (g *DbHandler) hideCoupletInternal()                      { g.coupletB.hide() }

func addAscByNumber(db *gorm.DB) *gorm.DB {
	return db.Order("couplets.number ASC")
}

func findByParent[T any](field string, parentId uint, order string) ([]T, error) {
	var out []T
	err := inits.DB.Where(field+" = ?", parentId).Order(order).Find(&out).Error
	return out, err
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
	return findByParent[models.Book]("translation_id", translationId, "number")
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
	return findByParent[models.Chapter]("book_id", bookId, "number")
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
	return findByParent[models.Verse]("chapter_id", chapterId, "number")
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

	return g.verseB.state
}

func (g *DbHandler) GetShownVerse() *ShownVerse {
	return g.verseB.state
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

	g.emit("songs_update", songs)
}

func (g *DbHandler) getCouplets(songId uint) ([]models.Couplet, error) {
	return findByParent[models.Couplet]("song_id", songId, "number ASC")
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

	return g.coupletB.state
}

func (g *DbHandler) CreateCouplet(text, label string, number, songId uint) {
	couplet := models.Couplet{
		Text:   text,
		Number: int(number),
		Label:  label,
		SongId: songId,
	}

	if err := inits.DB.Model(&models.Couplet{}).Where(
		"song_id = ? AND number >= ?", songId, number,
	).Update(
		"number", gorm.Expr("number + 1"),
	).Error; err != nil {
		log.Println("Error updating couplet numbers", err.Error())
		return
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
	g.emit("song_update", song)
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

	g.emit("song_update", song)
}

func (g *DbHandler) RemoveCouplet(coupletId int) {
	couplet := models.Couplet{}
	if err := inits.DB.Find(&couplet, coupletId).Error; err != nil {
		log.Println("Error finding couplet", err.Error())
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
	for i, c := range song.Couplets {
		c.Number = i + 1
		if err := inits.DB.Save(&c).Error; err != nil {
			log.Println("Error updating couplet number", err.Error())
			return
		}
	}
	g.emit("song_update", song)
}

func (g *DbHandler) GetShownCouplet() *ShownCouplet {
	return g.coupletB.state
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

func (g *DbHandler) GetCurrentScreenID() string {
	if g.app == nil {
		return ""
	}
	w, ok := g.app.Window.GetByName(mainWindowName)
	if !ok {
		return ""
	}
	scr, err := w.GetScreen()
	if err != nil || scr == nil {
		return ""
	}
	return scr.ID
}

func (g *DbHandler) ShowScreen(x, y, sizeX, sizeY float32, name string) {
	app := application.Get()

	w := app.Window.NewWithOptions(application.WebviewWindowOptions{
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
		KeyBindings: map[string]func(application.Window){
			"Escape": func(win application.Window) { win.Close() },
		},
		URL: "http://localhost:9093",
	})

	w.RegisterHook(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		g.emit("screen_closed", name)
	})
}

func (g *DbHandler) CloseScreen(name string) {
	app := application.Get()

	s, ok := app.Window.GetByName(name)
	if !ok {
		return
	}
	s.Close()
}
