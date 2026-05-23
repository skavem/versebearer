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

type StyleEvent struct {
	Type   string         `json:"type"`
	Target string         `json:"target,omitempty"`
	Style  map[string]any `json:"style,omitempty"`
	Fonts  []models.Font  `json:"fonts,omitempty"`
}

type StyleInput struct {
	BgColor      *string  `json:"bgColor"`
	BgOpacity    *float64 `json:"bgOpacity"`
	TextColor    *string  `json:"textColor"`
	FontId       *uint    `json:"fontId"`
	BorderColor  *string  `json:"borderColor"`
	BorderWidth  *int     `json:"borderWidth"`
	BorderRadius *int     `json:"borderRadius"`
	BorderStyle  *string  `json:"borderStyle"`
	Padding      *int     `json:"padding"`
	TextShadow   *string  `json:"textShadow"`
}

type VisualStyle struct {
	BgColor      string  `json:"bgColor"`
	BgOpacity    float64 `json:"bgOpacity"`
	TextColor    string  `json:"textColor"`
	FontId       *uint   `json:"fontId"`
	BorderColor  string  `json:"borderColor"`
	BorderWidth  int     `json:"borderWidth"`
	BorderRadius int     `json:"borderRadius"`
	BorderStyle  string  `json:"borderStyle"`
	Padding      int     `json:"padding"`
	TextShadow   string  `json:"textShadow"`
}

type VisualSettings struct {
	VerseStyle   VisualStyle   `json:"verseStyle"`
	CoupletStyle VisualStyle   `json:"coupletStyle"`
	Fonts        []models.Font `json:"fonts"`
}

var DefaultVerseStyle = VisualStyle{
	BgColor:      "#000000",
	BgOpacity:    0.95,
	TextColor:    "#ffffff",
	BorderColor:  "#000000",
	BorderWidth:  0,
	BorderRadius: 16,
	BorderStyle:  "solid",
	Padding:      32,
	TextShadow:   "",
}

var DefaultCoupletStyle = VisualStyle{
	BgColor:      "#000000",
	BgOpacity:    0.95,
	TextColor:    "#ffffff",
	BorderColor:  "#000000",
	BorderWidth:  0,
	BorderRadius: 0,
	BorderStyle:  "solid",
	Padding:      64,
	TextShadow:   "",
}

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

type CoupletInput struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

type DbHandler struct {
	verseB   *broadcaster[ShownVerse]
	coupletB *broadcaster[ShownCouplet]

	qr     chan *bool
	styleB chan *StyleEvent

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

func (g *DbHandler) CreateSong(number int, title string) *models.Song {
	dbSong := models.Song{Number: number, Title: title}
	err := inits.DB.Create(&dbSong).Error
	if err != nil {
		log.Println("Error creating song", err.Error())
		return nil
	}

	songs := g.GetSongs()
	if len(songs) > 0 {
		firstSongCouplets := g.GetCouplets(float32(songs[0].ID))
		songs[0].Couplets = firstSongCouplets
	}

	g.emit("songs_update", songs)
	return &dbSong
}

func (g *DbHandler) RemoveSong(songId int) {
	song := models.Song{}
	if err := inits.DB.Find(&song, songId).Error; err != nil {
		log.Println("Error finding song", err.Error())
		return
	}

	if g.coupletB.state != nil && g.coupletB.state.Song.ID == uint(songId) {
		g.hideCoupletInternal()
	}

	if err := inits.DB.Where("song_id = ?", songId).Delete(&models.Couplet{}).Error; err != nil {
		log.Println("Error deleting couplets for song", err.Error())
		return
	}

	if err := inits.DB.Delete(&models.Song{}, songId).Error; err != nil {
		log.Println("Error deleting song", err.Error())
		return
	}

	songs := g.GetSongs()
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

func (g *DbHandler) ReplaceCouplets(songId int, blocks []CoupletInput) {
	song := models.Song{}
	if err := inits.DB.Find(&song, songId).Error; err != nil {
		log.Println("Error finding song", err.Error())
		return
	}

	if g.coupletB.state != nil && g.coupletB.state.Song.ID == uint(songId) {
		g.hideCoupletInternal()
	}

	err := inits.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("song_id = ?", songId).Delete(&models.Couplet{}).Error; err != nil {
			return err
		}
		for i, b := range blocks {
			c := models.Couplet{
				Text:   b.Text,
				Label:  b.Label,
				Number: i + 1,
				SongId: uint(songId),
			}
			if err := tx.Create(&c).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Println("Error replacing couplets", err.Error())
		return
	}

	updatedSong := models.Song{}
	if err := inits.DB.Preload("Couplets", addAscByNumber).Find(&updatedSong, songId).Error; err != nil {
		log.Println("Error getting updated song", err.Error())
		return
	}
	g.emit("song_update", updatedSong)
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

// --- Visual / style handlers ---

func visualStyleFromGS(gs models.GlobalState, target string) VisualStyle {
	if target == "verse" {
		return VisualStyle{
			BgColor:      gs.VerseBgColor,
			BgOpacity:    gs.VerseBgOpacity,
			TextColor:    gs.VerseTextColor,
			FontId:       gs.VerseFontId,
			BorderColor:  gs.VerseBorderColor,
			BorderWidth:  gs.VerseBorderWidth,
			BorderRadius: gs.VerseBorderRadius,
			BorderStyle:  gs.VerseBorderStyle,
			Padding:      gs.VersePadding,
			TextShadow:   gs.VerseTextShadow,
		}
	}
	return VisualStyle{
		BgColor:      gs.CoupletBgColor,
		BgOpacity:    gs.CoupletBgOpacity,
		TextColor:    gs.CoupletTextColor,
		FontId:       gs.CoupletFontId,
		BorderColor:  gs.CoupletBorderColor,
		BorderWidth:  gs.CoupletBorderWidth,
		BorderRadius: gs.CoupletBorderRadius,
		BorderStyle:  gs.CoupletBorderStyle,
		Padding:      gs.CoupletPadding,
		TextShadow:   gs.CoupletTextShadow,
	}
}

func (g *DbHandler) GetVisualSettings() VisualSettings {
	gs := models.GlobalState{}
	if err := inits.DB.First(&gs, 1).Error; err != nil {
		log.Println("GetVisualSettings: error reading GlobalState", err)
	}
	var fonts []models.Font
	inits.DB.Find(&fonts)
	return VisualSettings{
		VerseStyle:   visualStyleFromGS(gs, "verse"),
		CoupletStyle: visualStyleFromGS(gs, "couplet"),
		Fonts:        fonts,
	}
}

func (g *DbHandler) UpdateVerseStyle(input StyleInput) VisualStyle {
	gs := models.GlobalState{}
	if err := inits.DB.First(&gs, 1).Error; err != nil {
		log.Println("UpdateVerseStyle: error reading GlobalState", err)
		return visualStyleFromGS(gs, "verse")
	}
	updates := map[string]any{}
	if input.BgColor != nil {
		gs.VerseBgColor = *input.BgColor
		updates["verse_bg_color"] = *input.BgColor
	}
	if input.BgOpacity != nil {
		gs.VerseBgOpacity = *input.BgOpacity
		updates["verse_bg_opacity"] = *input.BgOpacity
	}
	if input.TextColor != nil {
		gs.VerseTextColor = *input.TextColor
		updates["verse_text_color"] = *input.TextColor
	}
	if input.FontId != nil {
		gs.VerseFontId = input.FontId
		updates["verse_font_id"] = input.FontId
	}
	if input.BorderColor != nil {
		gs.VerseBorderColor = *input.BorderColor
		updates["verse_border_color"] = *input.BorderColor
	}
	if input.BorderWidth != nil {
		gs.VerseBorderWidth = *input.BorderWidth
		updates["verse_border_width"] = *input.BorderWidth
	}
	if input.BorderRadius != nil {
		gs.VerseBorderRadius = *input.BorderRadius
		updates["verse_border_radius"] = *input.BorderRadius
	}
	if input.BorderStyle != nil {
		gs.VerseBorderStyle = *input.BorderStyle
		updates["verse_border_style"] = *input.BorderStyle
	}
	if input.Padding != nil {
		gs.VersePadding = *input.Padding
		updates["verse_padding"] = *input.Padding
	}
	if input.TextShadow != nil {
		gs.VerseTextShadow = *input.TextShadow
		updates["verse_text_shadow"] = *input.TextShadow
	}
	if len(updates) > 0 {
		if err := inits.DB.Model(&gs).Updates(updates).Error; err != nil {
			log.Println("UpdateVerseStyle: error saving", err)
		}
	}
	style := visualStyleFromGS(gs, "verse")
	styleMap := map[string]any{
		"bgColor": style.BgColor, "bgOpacity": style.BgOpacity,
		"textColor": style.TextColor, "fontId": style.FontId,
		"borderColor": style.BorderColor, "borderWidth": style.BorderWidth,
		"borderRadius": style.BorderRadius, "borderStyle": style.BorderStyle,
		"padding": style.Padding, "textShadow": style.TextShadow,
	}
	g.styleB <- &StyleEvent{Type: "style_update", Target: "verse", Style: styleMap}
	return style
}

func (g *DbHandler) UpdateCoupletStyle(input StyleInput) VisualStyle {
	gs := models.GlobalState{}
	if err := inits.DB.First(&gs, 1).Error; err != nil {
		log.Println("UpdateCoupletStyle: error reading GlobalState", err)
		return visualStyleFromGS(gs, "couplet")
	}
	updates := map[string]any{}
	if input.BgColor != nil {
		gs.CoupletBgColor = *input.BgColor
		updates["couplet_bg_color"] = *input.BgColor
	}
	if input.BgOpacity != nil {
		gs.CoupletBgOpacity = *input.BgOpacity
		updates["couplet_bg_opacity"] = *input.BgOpacity
	}
	if input.TextColor != nil {
		gs.CoupletTextColor = *input.TextColor
		updates["couplet_text_color"] = *input.TextColor
	}
	if input.FontId != nil {
		gs.CoupletFontId = input.FontId
		updates["couplet_font_id"] = input.FontId
	}
	if input.BorderColor != nil {
		gs.CoupletBorderColor = *input.BorderColor
		updates["couplet_border_color"] = *input.BorderColor
	}
	if input.BorderWidth != nil {
		gs.CoupletBorderWidth = *input.BorderWidth
		updates["couplet_border_width"] = *input.BorderWidth
	}
	if input.BorderRadius != nil {
		gs.CoupletBorderRadius = *input.BorderRadius
		updates["couplet_border_radius"] = *input.BorderRadius
	}
	if input.BorderStyle != nil {
		gs.CoupletBorderStyle = *input.BorderStyle
		updates["couplet_border_style"] = *input.BorderStyle
	}
	if input.Padding != nil {
		gs.CoupletPadding = *input.Padding
		updates["couplet_padding"] = *input.Padding
	}
	if input.TextShadow != nil {
		gs.CoupletTextShadow = *input.TextShadow
		updates["couplet_text_shadow"] = *input.TextShadow
	}
	if len(updates) > 0 {
		if err := inits.DB.Model(&gs).Updates(updates).Error; err != nil {
			log.Println("UpdateCoupletStyle: error saving", err)
		}
	}
	style := visualStyleFromGS(gs, "couplet")
	styleMap := map[string]any{
		"bgColor": style.BgColor, "bgOpacity": style.BgOpacity,
		"textColor": style.TextColor, "fontId": style.FontId,
		"borderColor": style.BorderColor, "borderWidth": style.BorderWidth,
		"borderRadius": style.BorderRadius, "borderStyle": style.BorderStyle,
		"padding": style.Padding, "textShadow": style.TextShadow,
	}
	g.styleB <- &StyleEvent{Type: "style_update", Target: "couplet", Style: styleMap}
	return style
}

func (g *DbHandler) ResetVerseStyle() VisualStyle {
	gs := models.GlobalState{}
	if err := inits.DB.First(&gs, 1).Error; err != nil {
		log.Println("ResetVerseStyle: error reading GlobalState", err)
	}
	d := DefaultVerseStyle
	if err := inits.DB.Model(&gs).Updates(map[string]any{
		"verse_bg_color":      d.BgColor,
		"verse_bg_opacity":    d.BgOpacity,
		"verse_text_color":    d.TextColor,
		"verse_font_id":       nil,
		"verse_border_color":  d.BorderColor,
		"verse_border_width":  d.BorderWidth,
		"verse_border_radius": d.BorderRadius,
		"verse_border_style":  d.BorderStyle,
		"verse_padding":       d.Padding,
		"verse_text_shadow":   d.TextShadow,
	}).Error; err != nil {
		log.Println("ResetVerseStyle: error saving", err)
	}
	styleMap := map[string]any{
		"bgColor": d.BgColor, "bgOpacity": d.BgOpacity,
		"textColor": d.TextColor, "fontId": (*uint)(nil),
		"borderColor": d.BorderColor, "borderWidth": d.BorderWidth,
		"borderRadius": d.BorderRadius, "borderStyle": d.BorderStyle,
		"padding": d.Padding, "textShadow": d.TextShadow,
	}
	g.styleB <- &StyleEvent{Type: "style_update", Target: "verse", Style: styleMap}
	return d
}

func (g *DbHandler) ResetCoupletStyle() VisualStyle {
	gs := models.GlobalState{}
	if err := inits.DB.First(&gs, 1).Error; err != nil {
		log.Println("ResetCoupletStyle: error reading GlobalState", err)
	}
	d := DefaultCoupletStyle
	if err := inits.DB.Model(&gs).Updates(map[string]any{
		"couplet_bg_color":      d.BgColor,
		"couplet_bg_opacity":    d.BgOpacity,
		"couplet_text_color":    d.TextColor,
		"couplet_font_id":       nil,
		"couplet_border_color":  d.BorderColor,
		"couplet_border_width":  d.BorderWidth,
		"couplet_border_radius": d.BorderRadius,
		"couplet_border_style":  d.BorderStyle,
		"couplet_padding":       d.Padding,
		"couplet_text_shadow":   d.TextShadow,
	}).Error; err != nil {
		log.Println("ResetCoupletStyle: error saving", err)
	}
	styleMap := map[string]any{
		"bgColor": d.BgColor, "bgOpacity": d.BgOpacity,
		"textColor": d.TextColor, "fontId": (*uint)(nil),
		"borderColor": d.BorderColor, "borderWidth": d.BorderWidth,
		"borderRadius": d.BorderRadius, "borderStyle": d.BorderStyle,
		"padding": d.Padding, "textShadow": d.TextShadow,
	}
	g.styleB <- &StyleEvent{Type: "style_update", Target: "couplet", Style: styleMap}
	return d
}

func (g *DbHandler) UploadFont(name, mimeType string, data []byte) *models.Font {
	if len(data) > 5*1024*1024 {
		log.Println("UploadFont: file too large", len(data))
		return nil
	}
	validMimes := map[string]bool{
		"font/woff2":                  true,
		"font/ttf":                    true,
		"application/font-woff2":      true,
		"application/x-font-ttf":      true,
		"application/octet-stream":    true,
	}
	if !validMimes[mimeType] {
		log.Println("UploadFont: invalid mime type", mimeType)
		return nil
	}
	font := models.Font{
		Name:      name,
		MimeType:  mimeType,
		Data:      data,
		SizeBytes: len(data),
	}
	if err := inits.DB.Create(&font).Error; err != nil {
		log.Println("UploadFont: error creating font", err)
		return nil
	}
	var fonts []models.Font
	inits.DB.Find(&fonts)
	g.styleB <- &StyleEvent{Type: "fonts_changed", Fonts: fonts}
	return &font
}

func (g *DbHandler) DeleteFont(idF float32) {
	id := uint(idF)
	if err := inits.DB.Delete(&models.Font{}, id).Error; err != nil {
		log.Println("DeleteFont: error deleting font", err)
		return
	}
	// Null out any style FK pointing to this font
	gs := models.GlobalState{}
	if err := inits.DB.First(&gs, 1).Error; err == nil {
		updates := map[string]any{}
		if gs.VerseFontId != nil && *gs.VerseFontId == id {
			updates["verse_font_id"] = nil
		}
		if gs.CoupletFontId != nil && *gs.CoupletFontId == id {
			updates["couplet_font_id"] = nil
		}
		if len(updates) > 0 {
			inits.DB.Model(&gs).Updates(updates)
			inits.DB.First(&gs, 1)
			verseMap := map[string]any{
				"bgColor": gs.VerseBgColor, "bgOpacity": gs.VerseBgOpacity,
				"textColor": gs.VerseTextColor, "fontId": gs.VerseFontId,
				"borderColor": gs.VerseBorderColor, "borderWidth": gs.VerseBorderWidth,
				"borderRadius": gs.VerseBorderRadius, "borderStyle": gs.VerseBorderStyle,
				"padding": gs.VersePadding, "textShadow": gs.VerseTextShadow,
			}
			g.styleB <- &StyleEvent{Type: "style_update", Target: "verse", Style: verseMap}
			coupletMap := map[string]any{
				"bgColor": gs.CoupletBgColor, "bgOpacity": gs.CoupletBgOpacity,
				"textColor": gs.CoupletTextColor, "fontId": gs.CoupletFontId,
				"borderColor": gs.CoupletBorderColor, "borderWidth": gs.CoupletBorderWidth,
				"borderRadius": gs.CoupletBorderRadius, "borderStyle": gs.CoupletBorderStyle,
				"padding": gs.CoupletPadding, "textShadow": gs.CoupletTextShadow,
			}
			g.styleB <- &StyleEvent{Type: "style_update", Target: "couplet", Style: coupletMap}
		}
	}
	var fonts []models.Font
	inits.DB.Find(&fonts)
	g.styleB <- &StyleEvent{Type: "fonts_changed", Fonts: fonts}
}

func (g *DbHandler) getFontDataInternal(id uint) ([]byte, string, error) {
	var f models.Font
	if err := inits.DB.First(&f, id).Error; err != nil {
		return nil, "", err
	}
	return f.Data, f.MimeType, nil
}
