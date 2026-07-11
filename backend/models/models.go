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

	// ActiveThemeId points at the Theme whose verse/couplet styles are
	// currently broadcast to the receiver. Edits in the Визуал tab write
	// straight into this theme. Nil only transiently before the default
	// theme is seeded (see backend/inits/db.go).
	ActiveThemeId *uint `json:"activeThemeId"`
}

// Theme is a named preset of verse+couplet output styles. Exactly one theme
// is active at a time (GlobalState.ActiveThemeId). The style fields mirror the
// former flat GlobalState columns 1:1, so the receiver's style_update payload
// is unchanged. This entity is also the groundwork for future per-output
// ("зал"/"стрим") themes — each output would carry its own ThemeId.
type Theme struct {
	gorm.Model
	Name      string `json:"name"`
	IsDefault bool   `json:"isDefault"`

	// Verse style
	VerseBgColor      string  `json:"verseBgColor"`
	VerseBgOpacity    float64 `json:"verseBgOpacity"`
	VerseTextColor    string  `json:"verseTextColor"`
	VerseFontId       *uint   `json:"verseFontId"`
	VerseBorderColor  string  `json:"verseBorderColor"`
	VerseBorderWidth  int     `json:"verseBorderWidth"`
	VerseBorderRadius int     `json:"verseBorderRadius"`
	VerseBorderStyle  string  `json:"verseBorderStyle"`
	VersePadding      int     `json:"versePadding"`
	VerseMargin       int     `json:"verseMargin"`
	VerseTextShadow   string  `json:"verseTextShadow"`
	// Couplet style
	CoupletBgColor      string  `json:"coupletBgColor"`
	CoupletBgOpacity    float64 `json:"coupletBgOpacity"`
	CoupletTextColor    string  `json:"coupletTextColor"`
	CoupletFontId       *uint   `json:"coupletFontId"`
	CoupletBorderColor  string  `json:"coupletBorderColor"`
	CoupletBorderWidth  int     `json:"coupletBorderWidth"`
	CoupletBorderRadius int     `json:"coupletBorderRadius"`
	CoupletBorderStyle  string  `json:"coupletBorderStyle"`
	CoupletPadding      int     `json:"coupletPadding"`
	CoupletMargin       int     `json:"coupletMargin"`
	CoupletTextShadow   string  `json:"coupletTextShadow"`

	// Backdrop — a single, always-on full-screen layer behind the text cards,
	// constant for the whole theme (not tied to verse/couplet visibility).
	// BgType is "none" | "gradient" | "image" (empty == none). BgGradient holds
	// the gradient as JSON the receiver rebuilds into CSS.
	BgType     string `json:"bgType"`
	BgGradient string `json:"bgGradient"`
	BgImageId  *uint  `json:"bgImageId"`
}

type Font struct {
	gorm.Model
	Name      string `json:"name"`
	MimeType  string `json:"mimeType"`
	Data      []byte `json:"-"`
	SizeBytes int    `json:"sizeBytes"`
}

// Image is an uploaded backdrop picture, served by /image/{id} (mirrors Font
// and /font/{id}). Referenced from a theme via Verse/CoupletBgImageId.
type Image struct {
	gorm.Model
	Name      string `json:"name"`
	MimeType  string `json:"mimeType"`
	Data      []byte `json:"-"`
	SizeBytes int    `json:"sizeBytes"`
}
