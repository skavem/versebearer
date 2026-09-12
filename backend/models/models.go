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

	// AudioDeviceId — последнее выбранное устройство вывода звука (malgo
	// DeviceID.String()). Пустая строка — «системное по умолчанию».
	AudioDeviceId string `json:"audioDeviceId"`
	// AudioVolume — общая громкость плеера, 0..1. default:1, а не 0: любой
	// путь создания GlobalState (в т.ч. будущий) обязан получить полную
	// громкость, а не тишину — ноль здесь означает именно тишину, не «не
	// задано».
	AudioVolume float64 `json:"audioVolume" gorm:"default:1"`
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

// Output is a named projection target ("Зал", "Стрим"): content (verse/couplet
// text, qr, fonts) is broadcast identically to every output, but each output
// resolves its own style/backdrop via ThemeId (see resolveOutputTheme in
// dbHandler.go). The Output row is the source of truth for that resolution —
// not any in-memory cache — since CRUD on outputs is an infrequent operator
// action, not a hot path.
type Output struct {
	gorm.Model
	Name        string `json:"name"`
	ThemeId     *uint  `json:"themeId"`     // theme rendering this output; nil => default theme
	ScreenID    string `json:"screenId"`    // Wails Screen.ID of the target monitor; "" = unassigned
	Transparent bool   `json:"transparent"` // transparent window for OBS capture

	// Mode selects how the projector window is presented: "display" (frameless,
	// always-on-top, fills the assigned monitor — the original behavior) or
	// "window" (a normal moveable/resizable window). Empty is treated as
	// "display" so pre-existing rows (seeded before this field existed) keep
	// their current behavior unchanged.
	Mode string `json:"mode"`

	// The remaining fields only apply in "window" mode; "display" mode ignores
	// them (it is always frameless + always-on-top, sized to the monitor).
	WinX        int  `json:"winX"`
	WinY        int  `json:"winY"`
	WinWidth    int  `json:"winWidth"`    // 0 => default 1280
	WinHeight   int  `json:"winHeight"`   // 0 => default 720
	WinPlaced   bool `json:"winPlaced"`   // true once WinX/WinY hold a real position (first move/resize)
	Frameless   bool `json:"frameless"`   // window mode: window has no OS frame
	AlwaysOnTop bool `json:"alwaysOnTop"` // window mode: window floats above others
}

// AudioTrack — фонограмма в медиатеке. Файл лежит на диске (paths.MediaDir),
// в базе только имя: блоб на 5–80 МБ GORM тянул бы в память целиком при
// каждом чтении списка — в отличие от Font.Data/Image.Data, которые мелкие.
// MimeType не заводится: он однозначно выводится из расширения в FileName —
// у Font/Image он нужен только потому, что они отдаются по HTTP.
type AudioTrack struct {
	gorm.Model
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	FileName   string `json:"fileName"` // <hash>.<ext> внутри MediaDir, не полный путь
	DurationMs int    `json:"durationMs"`
	SizeBytes  int64  `json:"sizeBytes"`
	SourcePath string `json:"sourcePath"`        // откуда импортировали: показать оператору и переимпортировать
	Hash       string `json:"hash" gorm:"index"` // sha256 исходника, первые 16 байт hex; даёт дедупликацию — ищется на каждом импорте

	// Trim и Gain живут на треке, а не на PlaylistItem: тишина в начале
	// минусовки — свойство файла, она одинакова в любом плейлисте.
	TrimStartMs int     `json:"trimStartMs"`
	TrimEndMs   int     `json:"trimEndMs"` // 0 = играть до конца
	GainDb      float64 `json:"gainDb"`
}

// Playlist — и служебный список, и «фон до служения»: разница только во
// флагах, второй сущности не нужно. Shuffle сознательно не заведён — решение
// пользователя, см. .omc/plans/audio-playlist.md.
type Playlist struct {
	gorm.Model
	Name        string         `json:"name"`
	AutoAdvance bool           `json:"autoAdvance"`
	Loop        bool           `json:"loop"`
	FadeMs      int            `json:"fadeMs"`
	Items       []PlaylistItem `json:"items"`
}

type PlaylistItem struct {
	gorm.Model
	PlaylistId uint       `json:"playlistId" gorm:"index"` // список плейлиста грузится по этому полю
	TrackId    uint       `json:"trackId" gorm:"index"`    // RemoveTrack чистит по этому полю
	Track      AudioTrack `json:"track"`
	Position   int        `json:"position"` // 1..n, как Couplet.Number
}
