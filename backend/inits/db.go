package inits

import (
	"changeme/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func init() {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	db.AutoMigrate(
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
	)

	// Ensure GlobalState row 1 exists
	gs := models.GlobalState{}
	db.FirstOrCreate(&gs, models.GlobalState{Model: gorm.Model{ID: 1}})

	// version < "4": output styles moved from flat GlobalState columns into a
	// first-class Theme. Seed the "По умолчанию" theme once and point
	// ActiveThemeId at it. For an upgraded DB the legacy verse_*/couplet_*
	// columns still exist — carry the user's tuned values over. For a fresh DB
	// those columns were never created — fall back to hardcoded defaults.
	if gs.Version < "4" {
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
	}

	DB = db
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
	}
}
