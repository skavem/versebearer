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
		&models.GlobalState{},
		&models.Font{},
	)

	// Ensure GlobalState row 1 exists and seed style defaults on first launch
	gs := models.GlobalState{}
	db.FirstOrCreate(&gs, models.GlobalState{Model: gorm.Model{ID: 1}})

	if gs.Version < "2" {
		db.Model(&gs).Updates(map[string]any{
			"verse_bg_color":       "#000000",
			"verse_bg_opacity":     0.95,
			"verse_text_color":     "#ffffff",
			"verse_font_id":        nil,
			"verse_border_color":   "#000000",
			"verse_border_width":   0,
			"verse_border_radius":  16,
			"verse_border_style":   "solid",
			"verse_padding":        32,
			"verse_text_shadow":    "",
			"couplet_bg_color":     "#000000",
			"couplet_bg_opacity":   0.95,
			"couplet_text_color":   "#ffffff",
			"couplet_font_id":      nil,
			"couplet_border_color": "#000000",
			"couplet_border_width": 0,
			"couplet_border_radius": 0,
			"couplet_border_style": "solid",
			"couplet_padding":      64,
			"couplet_text_shadow":  "",
			"version":              "2",
		})
	}

	DB = db
}
