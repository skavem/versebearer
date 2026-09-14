package inits

import (
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
