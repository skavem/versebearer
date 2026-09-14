package inits

import (
	"fmt"
	"log"
	"strconv"

	"changeme/backend/models"
	"changeme/backend/paths"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

// versionLT сравнивает GlobalState.Version с числом n. Раньше сравнение шло
// как со строками ("10" < "4" — истина: байт '1' меньше '4'), и на версии 10
// все блоки миграции ниже начали бы гоняться заново при каждом запуске —
// например, блок "< 4" молча подменял бы настроенную оператором тему. Пустая
// или нечисловая строка трактуется как 0, чтобы свежая БД по-прежнему
// проходила все блоки миграции.
func versionLT(v string, n int) bool {
	parsed, err := strconv.Atoi(v)
	if err != nil {
		parsed = 0
	}
	return parsed < n
}

// Open вычисляет путь к базе данных (paths.DBPath()), при необходимости
// переносит найденную рядом с программой старую test.db (см. migrate.go) и
// открывает базу по итоговому пути через OpenAt. Вызывается явно из main() —
// НЕ из package init(): package init() отрабатывает до TestMain, то есть
// VERSEBEARER_DATA, выставленная тестом, опоздала бы, а «go test», набранный
// руками, продолжал бы открывать живую базу оператора при простом импорте
// пакета. См. TestNoInitFunc, который пинит отсутствие init() в этом пакете.
func Open() error {
	dbPath, err := paths.DBPath()
	if err != nil {
		return fmt.Errorf("не удалось определить путь к базе данных: %w", err)
	}

	if _, err := migrateLegacyDB(dbPath, legacyDBCandidates()); err != nil {
		return err
	}

	return OpenAt(dbPath)
}

// OpenAt открывает базу по явно заданному пути, прогоняет AutoMigrate и все
// блоки версионных миграций и на успехе присваивает пакетную переменную DB.
// Явный путь параметром (а не через paths) — чтобы OpenAt можно было
// протестировать на временных файлах, не трогая ни paths, ни настоящий
// AppData.
func OpenAt(path string) error {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return &OpenFailedError{Path: path, Err: err}
	}

	if err := db.AutoMigrate(
		&models.Translation{},
		&models.Book{},
		&models.Chapter{},
		&models.Verse{},
		&models.Song{},
		&models.Couplet{},
		&models.Screen{},
		&models.Theme{},
		&models.GlobalState{},
		&models.Font{},
		&models.Image{},
		&models.Output{},
		&models.AudioTrack{},
		&models.Playlist{},
		&models.PlaylistItem{},
	); err != nil {
		return &OpenFailedError{Path: path, Err: fmt.Errorf("не удалось обновить схему: %w", err)}
	}

	// Ensure GlobalState row 1 exists
	gs := models.GlobalState{}
	db.FirstOrCreate(&gs, models.GlobalState{Model: gorm.Model{ID: 1}})

	// version < 4: output styles moved from flat GlobalState columns into a
	// first-class Theme. Seed the "По умолчанию" theme once and point
	// ActiveThemeId at it. For an upgraded DB the legacy verse_*/couplet_*
	// columns still exist — carry the user's tuned values over. For a fresh DB
	// those columns were never created — fall back to hardcoded defaults.
	if versionLT(gs.Version, 4) {
		var seed models.Theme
		if db.Migrator().HasColumn(&models.GlobalState{}, "verse_bg_color") {
			// Legacy style columns still live on global_states — GORM maps them
			// straight into the Theme's matching columns.
			db.Table("global_states").Where("id = ?", 1).Scan(&seed)
			seed.Model = gorm.Model{}
		} else {
			seed = defaultTheme()
		}
		seed.Name = "По умолчанию"
		seed.IsDefault = true
		db.Create(&seed)
		db.Model(&gs).Updates(map[string]any{
			"active_theme_id": seed.ID,
			"version":         "4",
		})
		// Keep the in-memory GlobalState in sync so the version<5 seed below
		// (which runs in the same init on a fresh DB) sees the right theme.
		gs.ActiveThemeId = &seed.ID
		gs.Version = "4"
	}

	// version < 5: output styles/backdrops moved from the single active theme
	// to per-Output ThemeId (see models.Output). Seed one default Output
	// ("Экран") pointing at the current active theme so the existing "нажал
	// Транслировать" behavior is preserved unchanged after upgrade.
	if versionLT(gs.Version, 5) {
		db.Create(&models.Output{
			Name:        "Экран",
			ThemeId:     gs.ActiveThemeId,
			Transparent: false,
			ScreenID:    "",
		})
		db.Model(&gs).Update("version", "5")
		gs.Version = "5"
	}

	// version < 6: Output gained a window mode (models.Output.Mode) alongside
	// the pre-existing frameless/always-on-top display mode. Existing rows
	// predate the column, so their Mode is "" — pin it to "display" explicitly
	// and give WinWidth/WinHeight sane defaults for if the operator ever
	// switches that row to window mode later.
	if versionLT(gs.Version, 6) {
		db.Model(&models.Output{}).Where("mode = ?", "").Updates(map[string]any{
			"mode":       "display",
			"win_width":  1280,
			"win_height": 720,
		})
		db.Model(&gs).Update("version", "6")
		gs.Version = "6"
	}

	// version < 7: GlobalState.AudioVolume получает default:1 в теге GORM, но
	// AutoMigrate не переписывает существующие строки — на апгрейде колонка
	// создаётся нулём, а ноль громкости означает тишину при первом открытии
	// вкладки «Звук».
	if versionLT(gs.Version, 7) {
		db.Model(&models.GlobalState{}).Where("audio_volume = ?", 0).Update("audio_volume", 1.0)
		db.Model(&gs).Update("version", "7")
		gs.Version = "7"
	}

	// version < 8: экран «Медиатека» убран — импорт теперь всегда идёт сразу
	// в текущий плейлист, и без хотя бы одного плейлиста импортировать стало
	// бы некуда. Сеет один плейлист по умолчанию, если плейлистов ещё нет —
	// см. seedDefaultPlaylist.
	if versionLT(gs.Version, 8) {
		seedDefaultPlaylist(db)
		db.Model(&gs).Update("version", "8")
		gs.Version = "8"
	}

	DB = db
	return nil
}

// seedDefaultPlaylist создаёт плейлист «Плейлист», если плейлистов в базе
// ещё нет вообще. Вынесена отдельно от миграционного блока (а не инлайнена
// внутрь versionLT(gs.Version, 8)), чтобы быть тестируемой без завязки на
// package init()/захардкоженный "test.db" (db_test.go). Проверка по count,
// а не по версии: на versionLT(gs.Version, 8) она и так выполнится ровно
// один раз за апгрейд, но count делает функцию идемпотентной и сама по
// себе — повторный вызов на уже заселённой базе (например, ручной тест) не
// плодит второй плейлист.
func seedDefaultPlaylist(db *gorm.DB) {
	var count int64
	if err := db.Model(&models.Playlist{}).Count(&count).Error; err != nil || count > 0 {
		return
	}
	if err := db.Create(&models.Playlist{Name: "Плейлист"}).Error; err != nil {
		// LOW обзора: версия всё равно поднимется до "8" сразу после
		// вызова (см. вызывающего в init()) — повтора не будет, поэтому
		// молчание здесь означало бы оставить приложение вообще без
		// плейлиста навсегда, без единого следа в логе почему.
		log.Println("seedDefaultPlaylist: error creating default playlist", err)
	}
}

// defaultTheme is the hardcoded style used to seed the default theme on a fresh
// install (mirrors DefaultVerseStyle/DefaultCoupletStyle in the main package).
func defaultTheme() models.Theme {
	return models.Theme{
		VerseBgColor:      "#000000",
		VerseBgOpacity:    0.95,
		VerseTextColor:    "#ffffff",
		VerseBorderColor:  "#000000",
		VerseBorderWidth:  0,
		VerseBorderRadius: 16,
		VerseBorderStyle:  "solid",
		VersePadding:      32,
		VerseMargin:       0,
		VerseTextShadow:   "",

		CoupletBgColor:      "#000000",
		CoupletBgOpacity:    0.95,
		CoupletTextColor:    "#ffffff",
		CoupletBorderColor:  "#000000",
		CoupletBorderWidth:  0,
		CoupletBorderRadius: 0,
		CoupletBorderStyle:  "solid",
		CoupletPadding:      64,
		CoupletMargin:       0,
		CoupletTextShadow:   "",

		BgType: "none",
	}
}
