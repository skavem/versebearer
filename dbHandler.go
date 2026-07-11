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
	Margin       *int     `json:"margin"`
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
	Margin       int     `json:"margin"`
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
	Margin:       0,
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
	Margin:       0,
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

// screenIDForWindow resolves which screen a window sits on by its center
// point rather than its bounding rectangle. On Windows a maximized window's
// rect (GetWindowRect) is inflated by the invisible resize border (~8px), so
// it spills a few pixels onto an adjacent monitor. The bounds-based
// GetScreen() then ties at distance 0 between both monitors and returns
// whichever is listed first (usually the primary), so a window maximized on
// the secondary monitor can be reported as being on the primary. The center
// point is unambiguously inside a single monitor.
func screenIDForWindow(w application.Window) string {
	b := w.Bounds()
	center := application.Point{
		X: b.X + b.Width/2,
		Y: b.Y + b.Height/2,
	}
	scr := application.ScreenNearestDipPoint(center)
	if scr == nil {
		return ""
	}
	return scr.ID
}

func (g *DbHandler) GetCurrentScreenID() string {
	if g.app == nil {
		return ""
	}
	w, ok := g.app.Window.GetByName(mainWindowName)
	if !ok {
		return ""
	}
	return screenIDForWindow(w)
}

func (g *DbHandler) ShowScreen(x, y, sizeX, sizeY float32, name string, transparent bool) {
	app := application.Get()

	url := "http://localhost:9093"
	opts := application.WebviewWindowOptions{
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
	}

	if transparent {
		// Overlay mode: a see-through window so the projected verse/couplet
		// text floats over whatever else is on that monitor. Mouse events
		// pass through to the app underneath. The receiver reads
		// ?transparent=1 and drops its opaque page background.
		opts.BackgroundType = application.BackgroundTypeTransparent
		opts.BackgroundColour = application.NewRGBA(0, 0, 0, 0)
		opts.IgnoreMouseEvents = true
		url += "?transparent=1"
	}
	opts.URL = url

	w := app.Window.NewWithOptions(opts)

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

func styleFromTheme(t models.Theme, target string) VisualStyle {
	if target == "verse" {
		return VisualStyle{
			BgColor:      t.VerseBgColor,
			BgOpacity:    t.VerseBgOpacity,
			TextColor:    t.VerseTextColor,
			FontId:       t.VerseFontId,
			BorderColor:  t.VerseBorderColor,
			BorderWidth:  t.VerseBorderWidth,
			BorderRadius: t.VerseBorderRadius,
			BorderStyle:  t.VerseBorderStyle,
			Padding:      t.VersePadding,
			Margin:       t.VerseMargin,
			TextShadow:   t.VerseTextShadow,
		}
	}
	return VisualStyle{
		BgColor:      t.CoupletBgColor,
		BgOpacity:    t.CoupletBgOpacity,
		TextColor:    t.CoupletTextColor,
		FontId:       t.CoupletFontId,
		BorderColor:  t.CoupletBorderColor,
		BorderWidth:  t.CoupletBorderWidth,
		BorderRadius: t.CoupletBorderRadius,
		BorderStyle:  t.CoupletBorderStyle,
		Padding:      t.CoupletPadding,
		Margin:       t.CoupletMargin,
		TextShadow:   t.CoupletTextShadow,
	}
}

// styleToMap renders a VisualStyle into the loosely-typed payload the receiver
// merges (see mergeStyle in sse.go). Central so every broadcast stays in sync.
func styleToMap(s VisualStyle) map[string]any {
	return map[string]any{
		"bgColor": s.BgColor, "bgOpacity": s.BgOpacity,
		"textColor": s.TextColor, "fontId": s.FontId,
		"borderColor": s.BorderColor, "borderWidth": s.BorderWidth,
		"borderRadius": s.BorderRadius, "borderStyle": s.BorderStyle,
		"padding": s.Padding, "margin": s.Margin, "textShadow": s.TextShadow,
	}
}

// loadActiveTheme returns the theme GlobalState.ActiveThemeId points at,
// falling back to the default theme if the pointer is unset or dangling.
func loadActiveTheme() (models.Theme, error) {
	gs := models.GlobalState{}
	if err := inits.DB.First(&gs, 1).Error; err != nil {
		return models.Theme{}, err
	}
	var t models.Theme
	if gs.ActiveThemeId != nil {
		if err := inits.DB.First(&t, *gs.ActiveThemeId).Error; err == nil {
			return t, nil
		}
	}
	if err := inits.DB.Where("is_default = ?", true).First(&t).Error; err != nil {
		return models.Theme{}, err
	}
	return t, nil
}

// broadcastTheme pushes both verse and couplet styles of t to the receiver
// using the existing style_update mechanism (receiver is unchanged).
func (g *DbHandler) broadcastTheme(t models.Theme) {
	g.styleB <- &StyleEvent{Type: "style_update", Target: "verse", Style: styleToMap(styleFromTheme(t, "verse"))}
	g.styleB <- &StyleEvent{Type: "style_update", Target: "couplet", Style: styleToMap(styleFromTheme(t, "couplet"))}
}

func (g *DbHandler) GetVisualSettings() VisualSettings {
	t, err := loadActiveTheme()
	if err != nil {
		log.Println("GetVisualSettings: error loading active theme", err)
	}
	var fonts []models.Font
	inits.DB.Find(&fonts)
	return VisualSettings{
		VerseStyle:   styleFromTheme(t, "verse"),
		CoupletStyle: styleFromTheme(t, "couplet"),
		Fonts:        fonts,
	}
}

func (g *DbHandler) UpdateVerseStyle(input StyleInput) VisualStyle {
	t, err := loadActiveTheme()
	if err != nil {
		log.Println("UpdateVerseStyle: error loading active theme", err)
		return styleFromTheme(t, "verse")
	}
	updates := map[string]any{}
	if input.BgColor != nil {
		t.VerseBgColor = *input.BgColor
		updates["verse_bg_color"] = *input.BgColor
	}
	if input.BgOpacity != nil {
		t.VerseBgOpacity = *input.BgOpacity
		updates["verse_bg_opacity"] = *input.BgOpacity
	}
	if input.TextColor != nil {
		t.VerseTextColor = *input.TextColor
		updates["verse_text_color"] = *input.TextColor
	}
	if input.FontId != nil {
		t.VerseFontId = input.FontId
		updates["verse_font_id"] = input.FontId
	}
	if input.BorderColor != nil {
		t.VerseBorderColor = *input.BorderColor
		updates["verse_border_color"] = *input.BorderColor
	}
	if input.BorderWidth != nil {
		t.VerseBorderWidth = *input.BorderWidth
		updates["verse_border_width"] = *input.BorderWidth
	}
	if input.BorderRadius != nil {
		t.VerseBorderRadius = *input.BorderRadius
		updates["verse_border_radius"] = *input.BorderRadius
	}
	if input.BorderStyle != nil {
		t.VerseBorderStyle = *input.BorderStyle
		updates["verse_border_style"] = *input.BorderStyle
	}
	if input.Padding != nil {
		t.VersePadding = *input.Padding
		updates["verse_padding"] = *input.Padding
	}
	if input.Margin != nil {
		t.VerseMargin = *input.Margin
		updates["verse_margin"] = *input.Margin
	}
	if input.TextShadow != nil {
		t.VerseTextShadow = *input.TextShadow
		updates["verse_text_shadow"] = *input.TextShadow
	}
	if len(updates) > 0 {
		if err := inits.DB.Model(&t).Updates(updates).Error; err != nil {
			log.Println("UpdateVerseStyle: error saving", err)
		}
	}
	style := styleFromTheme(t, "verse")
	g.styleB <- &StyleEvent{Type: "style_update", Target: "verse", Style: styleToMap(style)}
	return style
}

func (g *DbHandler) UpdateCoupletStyle(input StyleInput) VisualStyle {
	t, err := loadActiveTheme()
	if err != nil {
		log.Println("UpdateCoupletStyle: error loading active theme", err)
		return styleFromTheme(t, "couplet")
	}
	updates := map[string]any{}
	if input.BgColor != nil {
		t.CoupletBgColor = *input.BgColor
		updates["couplet_bg_color"] = *input.BgColor
	}
	if input.BgOpacity != nil {
		t.CoupletBgOpacity = *input.BgOpacity
		updates["couplet_bg_opacity"] = *input.BgOpacity
	}
	if input.TextColor != nil {
		t.CoupletTextColor = *input.TextColor
		updates["couplet_text_color"] = *input.TextColor
	}
	if input.FontId != nil {
		t.CoupletFontId = input.FontId
		updates["couplet_font_id"] = input.FontId
	}
	if input.BorderColor != nil {
		t.CoupletBorderColor = *input.BorderColor
		updates["couplet_border_color"] = *input.BorderColor
	}
	if input.BorderWidth != nil {
		t.CoupletBorderWidth = *input.BorderWidth
		updates["couplet_border_width"] = *input.BorderWidth
	}
	if input.BorderRadius != nil {
		t.CoupletBorderRadius = *input.BorderRadius
		updates["couplet_border_radius"] = *input.BorderRadius
	}
	if input.BorderStyle != nil {
		t.CoupletBorderStyle = *input.BorderStyle
		updates["couplet_border_style"] = *input.BorderStyle
	}
	if input.Padding != nil {
		t.CoupletPadding = *input.Padding
		updates["couplet_padding"] = *input.Padding
	}
	if input.Margin != nil {
		t.CoupletMargin = *input.Margin
		updates["couplet_margin"] = *input.Margin
	}
	if input.TextShadow != nil {
		t.CoupletTextShadow = *input.TextShadow
		updates["couplet_text_shadow"] = *input.TextShadow
	}
	if len(updates) > 0 {
		if err := inits.DB.Model(&t).Updates(updates).Error; err != nil {
			log.Println("UpdateCoupletStyle: error saving", err)
		}
	}
	style := styleFromTheme(t, "couplet")
	g.styleB <- &StyleEvent{Type: "style_update", Target: "couplet", Style: styleToMap(style)}
	return style
}

func (g *DbHandler) ResetVerseStyle() VisualStyle {
	t, err := loadActiveTheme()
	if err != nil {
		log.Println("ResetVerseStyle: error loading active theme", err)
		return DefaultVerseStyle
	}
	d := DefaultVerseStyle
	if err := inits.DB.Model(&t).Updates(map[string]any{
		"verse_bg_color":      d.BgColor,
		"verse_bg_opacity":    d.BgOpacity,
		"verse_text_color":    d.TextColor,
		"verse_font_id":       nil,
		"verse_border_color":  d.BorderColor,
		"verse_border_width":  d.BorderWidth,
		"verse_border_radius": d.BorderRadius,
		"verse_border_style":  d.BorderStyle,
		"verse_padding":       d.Padding,
		"verse_margin":        d.Margin,
		"verse_text_shadow":   d.TextShadow,
	}).Error; err != nil {
		log.Println("ResetVerseStyle: error saving", err)
	}
	g.styleB <- &StyleEvent{Type: "style_update", Target: "verse", Style: styleToMap(d)}
	return d
}

func (g *DbHandler) ResetCoupletStyle() VisualStyle {
	t, err := loadActiveTheme()
	if err != nil {
		log.Println("ResetCoupletStyle: error loading active theme", err)
		return DefaultCoupletStyle
	}
	d := DefaultCoupletStyle
	if err := inits.DB.Model(&t).Updates(map[string]any{
		"couplet_bg_color":      d.BgColor,
		"couplet_bg_opacity":    d.BgOpacity,
		"couplet_text_color":    d.TextColor,
		"couplet_font_id":       nil,
		"couplet_border_color":  d.BorderColor,
		"couplet_border_width":  d.BorderWidth,
		"couplet_border_radius": d.BorderRadius,
		"couplet_border_style":  d.BorderStyle,
		"couplet_padding":       d.Padding,
		"couplet_margin":        d.Margin,
		"couplet_text_shadow":   d.TextShadow,
	}).Error; err != nil {
		log.Println("ResetCoupletStyle: error saving", err)
	}
	g.styleB <- &StyleEvent{Type: "style_update", Target: "couplet", Style: styleToMap(d)}
	return d
}

// --- Theme CRUD ---

func (g *DbHandler) ListThemes() []models.Theme {
	var themes []models.Theme
	if err := inits.DB.Order("is_default DESC, id ASC").Find(&themes).Error; err != nil {
		log.Println("ListThemes: error", err)
		return nil
	}
	return themes
}

// GetActiveThemeId returns the id of the currently active theme (0 on error).
func (g *DbHandler) GetActiveThemeId() uint {
	gs := models.GlobalState{}
	if err := inits.DB.First(&gs, 1).Error; err != nil || gs.ActiveThemeId == nil {
		return 0
	}
	return *gs.ActiveThemeId
}

// applyTheme sets the active theme and broadcasts its styles. Shared by
// ApplyTheme and the create/duplicate paths (a new copy becomes active so the
// operator edits it right away).
func (g *DbHandler) applyTheme(t models.Theme) {
	gs := models.GlobalState{}
	if err := inits.DB.First(&gs, 1).Error; err != nil {
		log.Println("applyTheme: error reading GlobalState", err)
		return
	}
	if err := inits.DB.Model(&gs).Update("active_theme_id", t.ID).Error; err != nil {
		log.Println("applyTheme: error saving active theme", err)
		return
	}
	g.broadcastTheme(t)
}

func (g *DbHandler) ApplyTheme(idF float32) []models.Theme {
	var t models.Theme
	if err := inits.DB.First(&t, uint(idF)).Error; err != nil {
		log.Println("ApplyTheme: theme not found", err)
		return g.ListThemes()
	}
	g.applyTheme(t)
	return g.ListThemes()
}

// copyThemeStyle clones the style fields of src into a fresh, non-default theme
// with the given name, persists it, makes it active and returns it.
func (g *DbHandler) copyThemeStyle(src models.Theme, name string) *models.Theme {
	nt := src
	nt.Model = gorm.Model{}
	nt.Name = name
	nt.IsDefault = false
	if err := inits.DB.Create(&nt).Error; err != nil {
		log.Println("copyThemeStyle: error creating theme", err)
		return nil
	}
	g.applyTheme(nt)
	return &nt
}

// CreateTheme adds a new theme as a copy of the currently active one.
func (g *DbHandler) CreateTheme(name string) *models.Theme {
	src, err := loadActiveTheme()
	if err != nil {
		log.Println("CreateTheme: error loading active theme", err)
		return nil
	}
	return g.copyThemeStyle(src, name)
}

// DuplicateTheme adds a new theme as a copy of the given theme.
func (g *DbHandler) DuplicateTheme(idF float32) *models.Theme {
	var src models.Theme
	if err := inits.DB.First(&src, uint(idF)).Error; err != nil {
		log.Println("DuplicateTheme: theme not found", err)
		return nil
	}
	return g.copyThemeStyle(src, src.Name+" (копия)")
}

func (g *DbHandler) RenameTheme(idF float32, name string) []models.Theme {
	if err := inits.DB.Model(&models.Theme{}).Where("id = ?", uint(idF)).Update("name", name).Error; err != nil {
		log.Println("RenameTheme: error", err)
	}
	return g.ListThemes()
}

// DeleteTheme removes a theme. The default theme cannot be deleted. Deleting the
// active theme falls back to the default theme (re-broadcasting its styles).
func (g *DbHandler) DeleteTheme(idF float32) []models.Theme {
	id := uint(idF)
	var t models.Theme
	if err := inits.DB.First(&t, id).Error; err != nil {
		log.Println("DeleteTheme: theme not found", err)
		return g.ListThemes()
	}
	if t.IsDefault {
		log.Println("DeleteTheme: refusing to delete default theme")
		return g.ListThemes()
	}
	wasActive := g.GetActiveThemeId() == id
	if err := inits.DB.Delete(&models.Theme{}, id).Error; err != nil {
		log.Println("DeleteTheme: error deleting", err)
		return g.ListThemes()
	}
	if wasActive {
		var def models.Theme
		if err := inits.DB.Where("is_default = ?", true).First(&def).Error; err == nil {
			g.applyTheme(def)
		}
	}
	return g.ListThemes()
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
	// Null out any theme style FK pointing to this font, across all themes.
	inits.DB.Model(&models.Theme{}).Where("verse_font_id = ?", id).Update("verse_font_id", nil)
	inits.DB.Model(&models.Theme{}).Where("couplet_font_id = ?", id).Update("couplet_font_id", nil)
	// Re-broadcast the active theme so a live projection using the deleted font
	// falls back immediately.
	if t, err := loadActiveTheme(); err == nil {
		g.broadcastTheme(t)
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
