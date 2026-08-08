package main

import (
	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/search"
	"github.com/wailsapp/wails/v3/pkg/application"
	"gorm.io/gorm"
	"sync"
	"time"
)

const mainWindowName = "VerseBearer"

type StyleEvent struct {
	Type   string         `json:"type"`
	Target string         `json:"target,omitempty"`
	Style  map[string]any `json:"style,omitempty"`
	Fonts  []models.Font  `json:"fonts,omitempty"`
	// ThemeId is the theme being edited when Type is style_update/backdrop_update.
	// watchChannels uses it to compute which outputs (outputIds) the change
	// should reach — see outputIdsForTheme in sse.go.
	ThemeId uint `json:"themeId,omitempty"`
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

// Backdrop is the theme's single always-on full-screen background layer.
type Backdrop struct {
	BgType     string `json:"bgType"`
	BgGradient string `json:"bgGradient"`
	BgImageId  *uint  `json:"bgImageId"`
}

type BackdropInput struct {
	BgType     *string `json:"bgType"`
	BgGradient *string `json:"bgGradient"`
	BgImageId  *uint   `json:"bgImageId"`
}

type VisualSettings struct {
	VerseStyle   VisualStyle    `json:"verseStyle"`
	CoupletStyle VisualStyle    `json:"coupletStyle"`
	Backdrop     Backdrop       `json:"backdrop"`
	Fonts        []models.Font  `json:"fonts"`
	Images       []models.Image `json:"images"`
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

	// searchIdx — полнотекстовый индекс стихов и куплетов (см. db_search.go).
	// Полностью производен от SQLite: пересобирается при пустом индексе на
	// старте, дальше поддерживается точечными хуками из мутаций куплетов и
	// импорта переводов. nil, если индекс не удалось открыть — тогда поиск
	// молча возвращает пустоту, а остальная программа работает как прежде.
	searchIdx *search.Index

	// rebuildMu не даёт запустить две полные пересборки индекса разом
	// (см. RebuildSearchIndex).
	rebuildMu sync.Mutex

	// winSaveTimers debounces WinX/WinY/WinWidth/WinHeight persistence for
	// window-mode outputs (see watchWindowGeometry) — one timer per output id,
	// reset on every move/resize event so a drag doesn't hammer SQLite.
	winSaveMu     sync.Mutex
	winSaveTimers map[uint]*time.Timer
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
