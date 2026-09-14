package inits

import (
	"path/filepath"
	"testing"

	"changeme/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestSeedDefaultPlaylistIdempotent — план, правка 1: миграция версии 8
// сеет плейлист «Плейлист», если плейлистов ещё нет, и не плодит второй при
// повторном вызове (в частности, на уже заселённой базе, которую снова
// открыли).
func TestSeedDefaultPlaylistIdempotent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Playlist{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	seedDefaultPlaylist(db)
	seedDefaultPlaylist(db)

	var playlists []models.Playlist
	if err := db.Find(&playlists).Error; err != nil {
		t.Fatalf("find playlists: %v", err)
	}
	if len(playlists) != 1 {
		t.Fatalf("expected exactly 1 seeded playlist after two calls, got %d", len(playlists))
	}
	if playlists[0].Name != "Плейлист" {
		t.Errorf("seeded playlist name = %q, want %q", playlists[0].Name, "Плейлист")
	}
}

// TestSeedDefaultPlaylistSkipsExisting — не создаёт "Плейлист", если
// оператор уже создал свои плейлисты до апгрейда: count>0 значит "плейлисты
// уже есть", а не "плейлист по умолчанию уже есть".
func TestSeedDefaultPlaylistSkipsExisting(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.Playlist{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	db.Create(&models.Playlist{Name: "Служебный"})

	seedDefaultPlaylist(db)

	var count int64
	db.Model(&models.Playlist{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected existing playlist count to stay 1, got %d", count)
	}
}

func TestVersionLT(t *testing.T) {
	cases := []struct {
		v    string
		n    int
		want bool
	}{
		{"", 4, true}, // пустая строка — свежая БД, должна пройти все блоки
		{"6", 7, true},
		{"10", 4, false}, // главный кейс бага: раньше "10" < "4" было true при строковом сравнении
		{"abc", 1, true}, // нечисловой мусор трактуется как 0
		{"7", 7, false},
	}
	for _, c := range cases {
		if got := versionLT(c.v, c.n); got != c.want {
			t.Errorf("versionLT(%q, %d) = %v, want %v", c.v, c.n, got, c.want)
		}
	}
}

// TestOpenAtOnUpgradedDBIsNoOp — Р5 ревизии. Самый дорогой шаг переноса — не
// копирование файла, а перенос ~105 строк блоков versionLT из бывшего
// init() в OpenAt: потерянный gs.Version = "4" или переставленный блок
// сбрасывает оператора на дефолты тихо, без единой ошибки (db.go:18 прямо
// об этом предупреждает). Повторный OpenAt на уже апгрейженной базе обязан
// быть строгим no-op: версия, активная тема и число строк во всех
// затронутых таблицах не должны меняться.
func TestOpenAtOnUpgradedDBIsNoOp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "versebearer.db")
	if err := OpenAt(path); err != nil {
		t.Fatalf("первый OpenAt: %v", err)
	}
	firstConn := DB

	var gs models.GlobalState
	if err := DB.First(&gs, 1).Error; err != nil {
		t.Fatalf("GlobalState после первого OpenAt: %v", err)
	}
	if gs.Version != "8" {
		t.Fatalf("version после первого OpenAt = %q, want %q", gs.Version, "8")
	}

	// Операторские правки поверх дефолтов: своя тема, свой Output, своя
	// активная тема.
	theme := models.Theme{Name: "Оператор", VerseBgColor: "#123456"}
	if err := DB.Create(&theme).Error; err != nil {
		t.Fatalf("create theme: %v", err)
	}
	if err := DB.Model(&gs).Update("active_theme_id", theme.ID).Error; err != nil {
		t.Fatalf("update active theme: %v", err)
	}
	if err := DB.Create(&models.Output{Name: "Мой экран", ThemeId: &theme.ID}).Error; err != nil {
		t.Fatalf("create output: %v", err)
	}

	var themesBefore, outputsBefore, playlistsBefore int64
	DB.Model(&models.Theme{}).Count(&themesBefore)
	DB.Model(&models.Output{}).Count(&outputsBefore)
	DB.Model(&models.Playlist{}).Count(&playlistsBefore)

	if err := OpenAt(path); err != nil {
		t.Fatalf("второй OpenAt: %v", err)
	}
	// Первое соединение больше не нужно, но держит хендл на файл — закрыть
	// до того, как t.TempDir() попробует его удалить (Windows иначе падает:
	// "The process cannot access the file because it is being used by
	// another process").
	closeGormDB(firstConn)
	t.Cleanup(func() { closeGormDB(DB) })

	var gs2 models.GlobalState
	if err := DB.First(&gs2, 1).Error; err != nil {
		t.Fatalf("GlobalState после второго OpenAt: %v", err)
	}
	if gs2.Version != "8" {
		t.Errorf("version после второго OpenAt = %q, want %q", gs2.Version, "8")
	}
	if gs2.ActiveThemeId == nil || *gs2.ActiveThemeId != theme.ID {
		t.Errorf("ActiveThemeId изменился: got %v, want %d — повторный OpenAt тихо сбросил операторскую тему", gs2.ActiveThemeId, theme.ID)
	}

	var themeAfter models.Theme
	if err := DB.First(&themeAfter, theme.ID).Error; err != nil {
		t.Fatalf("theme после второго OpenAt: %v", err)
	}
	if themeAfter.VerseBgColor != "#123456" {
		t.Errorf("тема оператора перезаписана: VerseBgColor = %q, want %q", themeAfter.VerseBgColor, "#123456")
	}

	var themesAfter, outputsAfter, playlistsAfter int64
	DB.Model(&models.Theme{}).Count(&themesAfter)
	DB.Model(&models.Output{}).Count(&outputsAfter)
	DB.Model(&models.Playlist{}).Count(&playlistsAfter)
	if themesAfter != themesBefore {
		t.Errorf("число тем изменилось: было %d, стало %d", themesBefore, themesAfter)
	}
	if outputsAfter != outputsBefore {
		t.Errorf("число Output изменилось: было %d, стало %d", outputsBefore, outputsAfter)
	}
	if playlistsAfter != playlistsBefore {
		t.Errorf("число плейлистов изменилось: было %d, стало %d", playlistsBefore, playlistsAfter)
	}
}
