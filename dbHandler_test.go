package main

import (
	"testing"

	"changeme/backend/inits"
	"changeme/backend/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.Translation{},
		&models.Book{},
		&models.Chapter{},
		&models.Verse{},
		&models.Song{},
		&models.Couplet{},
	); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	inits.DB = db
}

func seedTwoSongs(t *testing.T) (uint, uint) {
	t.Helper()
	songA := &models.Song{Title: "A", Number: 1}
	songB := &models.Song{Title: "B", Number: 2}
	if err := inits.DB.Create(songA).Error; err != nil {
		t.Fatal(err)
	}
	if err := inits.DB.Create(songB).Error; err != nil {
		t.Fatal(err)
	}
	for _, sid := range []uint{songA.ID, songB.ID} {
		for i := 1; i <= 3; i++ {
			c := &models.Couplet{SongId: sid, Number: i, Text: "x", Label: "v"}
			if err := inits.DB.Create(c).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	return songA.ID, songB.ID
}

func coupletsBySong(t *testing.T, songId uint) []models.Couplet {
	t.Helper()
	var out []models.Couplet
	if err := inits.DB.Where("song_id = ?", songId).Order("number ASC").Find(&out).Error; err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCreateCouplet_ShiftsOnlyOwnSong(t *testing.T) {
	setupTestDB(t)
	aID, bID := seedTwoSongs(t)

	g := &DbHandler{}
	g.CreateCouplet("new", "v", 2, aID)

	a := coupletsBySong(t, aID)
	b := coupletsBySong(t, bID)

	if len(a) != 4 {
		t.Fatalf("song A: want 4 couplets, got %d", len(a))
	}
	for i, c := range a {
		if c.Number != i+1 {
			t.Errorf("song A[%d].Number = %d, want %d", i, c.Number, i+1)
		}
	}

	if len(b) != 3 {
		t.Fatalf("song B: want 3 couplets (untouched), got %d", len(b))
	}
	for i, c := range b {
		if c.Number != i+1 {
			t.Errorf("song B[%d].Number = %d, want %d (must be untouched)", i, c.Number, i+1)
		}
	}
}

func TestRemoveCouplet_RenumbersWithinSong(t *testing.T) {
	setupTestDB(t)
	aID, bID := seedTwoSongs(t)

	a := coupletsBySong(t, aID)
	middleID := a[1].ID

	g := &DbHandler{}
	g.RemoveCouplet(int(middleID))

	after := coupletsBySong(t, aID)
	if len(after) != 2 {
		t.Fatalf("song A: want 2 couplets after remove, got %d", len(after))
	}
	for i, c := range after {
		if c.Number != i+1 {
			t.Errorf("song A[%d].Number = %d, want %d", i, c.Number, i+1)
		}
	}

	b := coupletsBySong(t, bID)
	if len(b) != 3 {
		t.Fatalf("song B: want 3 couplets (untouched), got %d", len(b))
	}
}

func TestGetTranslations_PreloadsOnlyFirstBranch(t *testing.T) {
	setupTestDB(t)

	tr1 := &models.Translation{Name: "tr1"}
	tr2 := &models.Translation{Name: "tr2"}
	if err := inits.DB.Create(tr1).Error; err != nil {
		t.Fatal(err)
	}
	if err := inits.DB.Create(tr2).Error; err != nil {
		t.Fatal(err)
	}

	for _, trID := range []uint{tr1.ID, tr2.ID} {
		for i := 1; i <= 2; i++ {
			b := &models.Book{TranslationId: trID, Number: i, Title: "b"}
			if err := inits.DB.Create(b).Error; err != nil {
				t.Fatal(err)
			}
			for j := 1; j <= 2; j++ {
				ch := &models.Chapter{BookId: b.ID, Number: j}
				if err := inits.DB.Create(ch).Error; err != nil {
					t.Fatal(err)
				}
				for k := 1; k <= 2; k++ {
					if err := inits.DB.Create(&models.Verse{ChapterId: ch.ID, Number: k, Text: "v"}).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	}

	g := &DbHandler{}
	translations := g.GetTranslations()

	if len(translations) != 2 {
		t.Fatalf("want 2 translations, got %d", len(translations))
	}

	if got := len(translations[0].Books); got != 2 {
		t.Errorf("translations[0].Books: want 2, got %d", got)
	}
	if got := len(translations[0].Books[0].Chapters); got != 2 {
		t.Errorf("translations[0].Books[0].Chapters: want 2, got %d", got)
	}
	if got := len(translations[0].Books[0].Chapters[0].Verses); got != 2 {
		t.Errorf("translations[0].Books[0].Chapters[0].Verses: want 2, got %d", got)
	}

	if got := len(translations[1].Books); got != 0 {
		t.Errorf("translations[1].Books: want 0 (lazy, not preloaded), got %d", got)
	}
	if got := len(translations[0].Books[1].Chapters); got != 0 {
		t.Errorf("translations[0].Books[1].Chapters: want 0 (lazy), got %d", got)
	}
}

const importFileJSON = `{
  "name": "Тестовый перевод",
  "shortName": "ТЕСТ",
  "source": "unit-test",
  "books": [
    {"dividerBefore": "Ветхий Завет", "name": "Быт", "fullName": "Бытие",
     "content": [["стих один", "стих два"], ["стих три"]]},
    {"name": "Мф", "fullName": "От Матфея",
     "content": [["стих четыре", "", "стих шесть"]]}
  ]
}`

// A file that is a bare array of books (the pre-header Bible.json layout) still
// has to import — the name then comes from the caller.
const importBareJSON = `[{"name": "Быт", "fullName": "Бытие", "content": [["текст"]]}]`

func TestImportTranslation_CountsAndSkipsEmptyVerses(t *testing.T) {
	setupTestDB(t)
	g := &DbHandler{}

	res := g.ImportTranslation("", "", []byte(importFileJSON))
	if res.Error != "" {
		t.Fatalf("import failed: %s", res.Error)
	}
	if res.Name != "Тестовый перевод" || res.ShortName != "ТЕСТ" {
		t.Errorf("header not picked up: %q / %q", res.Name, res.ShortName)
	}
	if res.Books != 2 || res.Chapters != 3 || res.Verses != 5 {
		t.Errorf("want 2 books / 3 chapters / 5 verses, got %d / %d / %d",
			res.Books, res.Chapters, res.Verses)
	}

	// The empty slot must not become a verse, and the verse after it keeps its
	// original number so references stay correct.
	var verses []models.Verse
	inits.DB.Joins("JOIN chapters ON chapters.id = verses.chapter_id").
		Joins("JOIN books ON books.id = chapters.book_id").
		Where("books.short_name = ?", "Мф").Order("verses.number").Find(&verses)
	if len(verses) != 2 {
		t.Fatalf("Мф: want 2 verses, got %d", len(verses))
	}
	if verses[1].Number != 3 {
		t.Errorf("verse after the gap: want number 3, got %d", verses[1].Number)
	}

	// Book order must survive the import — getBooks sorts by number.
	var books []models.Book
	inits.DB.Where("translation_id = ?", res.TranslationId).Order("number").Find(&books)
	if len(books) != 2 || books[0].ShortName != "Быт" || books[1].Number != 2 {
		t.Errorf("book numbering broken: %+v", books)
	}
}

func TestImportTranslation_RejectsDuplicateName(t *testing.T) {
	setupTestDB(t)
	g := &DbHandler{}

	if res := g.ImportTranslation("", "", []byte(importFileJSON)); res.Error != "" {
		t.Fatalf("first import failed: %s", res.Error)
	}
	res := g.ImportTranslation("", "", []byte(importFileJSON))
	if res.Error == "" {
		t.Fatal("second import with the same name should fail")
	}

	var count int64
	inits.DB.Model(&models.Translation{}).Count(&count)
	if count != 1 {
		t.Errorf("want 1 translation after the rejected import, got %d", count)
	}
}

func TestImportTranslation_BareListNeedsName(t *testing.T) {
	setupTestDB(t)
	g := &DbHandler{}

	if res := g.ImportTranslation("", "", []byte(importBareJSON)); res.Error == "" {
		t.Error("headerless file without a name should be rejected")
	}
	res := g.ImportTranslation("Мой перевод", "МОЙ", []byte(importBareJSON))
	if res.Error != "" {
		t.Fatalf("named import of a headerless file failed: %s", res.Error)
	}
	if res.Name != "Мой перевод" || res.Verses != 1 {
		t.Errorf("unexpected result: %+v", res)
	}
}

func TestRemoveTranslation_CascadesAndKeepsLastOne(t *testing.T) {
	setupTestDB(t)
	g := &DbHandler{}

	first := g.ImportTranslation("Первый", "ОДИН", []byte(importBareJSON))
	if msg := g.RemoveTranslation(float32(first.TranslationId)); msg == "" {
		t.Error("removing the only translation should be refused")
	}

	second := g.ImportTranslation("", "", []byte(importFileJSON))
	if msg := g.RemoveTranslation(float32(second.TranslationId)); msg != "" {
		t.Fatalf("remove failed: %s", msg)
	}

	var books, chapters, verses int64
	inits.DB.Model(&models.Book{}).Where("translation_id = ?", second.TranslationId).Count(&books)
	inits.DB.Model(&models.Chapter{}).Count(&chapters)
	inits.DB.Model(&models.Verse{}).Count(&verses)
	if books != 0 {
		t.Errorf("books left behind: %d", books)
	}
	// Only the surviving translation's single chapter/verse may remain.
	if chapters != 1 || verses != 1 {
		t.Errorf("cascade leaked rows: chapters %d, verses %d", chapters, verses)
	}
}
