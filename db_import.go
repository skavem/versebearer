package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"changeme/backend/inits"
	"changeme/backend/models"
)

// TranslationFileBook is one book inside an import file: a short name, a full
// title, an optional section divider and the verse text itself as
// content[chapter][verse]. Empty strings inside content mark verses the
// translation does not have — they keep the numbering aligned and are skipped
// on import.
type TranslationFileBook struct {
	DividerBefore string     `json:"dividerBefore"`
	Name          string     `json:"name"`
	FullName      string     `json:"fullName"`
	Content       [][]string `json:"content"`
}

// TranslationFile is the import file itself. The books array is what
// backend/filler has always consumed; name/shortName/source are the header that
// makes the file self-describing, so the operator does not have to type the
// translation name by hand.
type TranslationFile struct {
	Name      string                `json:"name"`
	ShortName string                `json:"shortName"`
	Source    string                `json:"source"`
	Books     []TranslationFileBook `json:"books"`
}

// ImportPreview is what the UI shows before the operator confirms an import.
type ImportPreview struct {
	Name       string `json:"name"`
	ShortName  string `json:"shortName"`
	Source     string `json:"source"`
	Books      int    `json:"books"`
	Chapters   int    `json:"chapters"`
	Verses     int    `json:"verses"`
	Duplicate  bool   `json:"duplicate"`
	FirstBook  string `json:"firstBook"`
	Error      string `json:"error"`
	NeedsName  bool   `json:"needsName"`
	IsBareList bool   `json:"isBareList"`
}

// ImportResult reports the outcome of an import. Error is empty on success.
type ImportResult struct {
	TranslationId uint   `json:"translationId"`
	Name          string `json:"name"`
	ShortName     string `json:"shortName"`
	Books         int    `json:"books"`
	Chapters      int    `json:"chapters"`
	Verses        int    `json:"verses"`
	Error         string `json:"error"`
}

// TranslationSummary is a row in the translation list shown in settings.
type TranslationSummary struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"shortName"`
	Books     int    `json:"books"`
	Verses    int    `json:"verses"`
	InUse     bool   `json:"inUse"`
}

// parseTranslationFile accepts both shapes: the self-describing object and a
// bare array of books (the older Bible.json layout), so files produced before
// the header existed still import.
func parseTranslationFile(data []byte) (*TranslationFile, bool, error) {
	//  — BOM, который редакторы на Windows дописывают в начало файла
	trimmed := strings.TrimLeft(string(data), " \t\r\n\ufeff")

	if strings.HasPrefix(trimmed, "[") {
		var books []TranslationFileBook
		if err := json.Unmarshal(data, &books); err != nil {
			return nil, true, fmt.Errorf("файл не читается как список книг: %w", err)
		}
		return &TranslationFile{Books: books}, true, nil
	}

	var file TranslationFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, false, fmt.Errorf("файл не читается: %w", err)
	}
	return &file, false, nil
}

func countFile(file *TranslationFile) (chapters, verses int) {
	for _, b := range file.Books {
		chapters += len(b.Content)
		for _, ch := range b.Content {
			for _, v := range ch {
				if v != "" {
					verses++
				}
			}
		}
	}
	return
}

// InspectTranslationFile validates a file and reports what would be imported.
// Nothing is written — the UI calls this first so the operator sees the
// translation name and size before committing.
func (g *DbHandler) InspectTranslationFile(data []byte) *ImportPreview {
	file, bare, err := parseTranslationFile(data)
	if err != nil {
		return &ImportPreview{Error: err.Error()}
	}
	if len(file.Books) == 0 {
		return &ImportPreview{Error: "в файле нет книг"}
	}

	chapters, verses := countFile(file)
	if verses == 0 {
		return &ImportPreview{Error: "в файле нет текста стихов"}
	}

	preview := &ImportPreview{
		Name:       file.Name,
		ShortName:  file.ShortName,
		Source:     file.Source,
		Books:      len(file.Books),
		Chapters:   chapters,
		Verses:     verses,
		FirstBook:  file.Books[0].FullName,
		NeedsName:  strings.TrimSpace(file.Name) == "",
		IsBareList: bare,
	}

	if preview.Name != "" {
		var existing int64
		inits.DB.Model(&models.Translation{}).Where("name = ?", preview.Name).Count(&existing)
		preview.Duplicate = existing > 0
	}

	return preview
}

// ImportTranslation writes a translation file into the DB. name/shortName
// override the file header (the UI passes what the operator sees, and supplies
// them outright for headerless files). Everything lands in one transaction:
// a failure leaves no half-imported translation behind.
func (g *DbHandler) ImportTranslation(name, shortName string, data []byte) *ImportResult {
	file, _, err := parseTranslationFile(data)
	if err != nil {
		return &ImportResult{Error: err.Error()}
	}

	if strings.TrimSpace(name) != "" {
		file.Name = strings.TrimSpace(name)
	}
	if strings.TrimSpace(shortName) != "" {
		file.ShortName = strings.TrimSpace(shortName)
	}
	if file.Name == "" {
		return &ImportResult{Error: "не указано название перевода"}
	}
	if len(file.Books) == 0 {
		return &ImportResult{Error: "в файле нет книг"}
	}

	var clash int64
	inits.DB.Model(&models.Translation{}).Where("name = ?", file.Name).Count(&clash)
	if clash > 0 {
		return &ImportResult{Error: "перевод «" + file.Name + "» уже установлен"}
	}

	result := &ImportResult{Name: file.Name, ShortName: file.ShortName}

	tx := inits.DB.Begin()
	if tx.Error != nil {
		return &ImportResult{Error: "не удалось начать запись: " + tx.Error.Error()}
	}

	translation := models.Translation{Name: file.Name, ShortName: file.ShortName}
	if err := tx.Create(&translation).Error; err != nil {
		tx.Rollback()
		return &ImportResult{Error: "не удалось создать перевод: " + err.Error()}
	}

	for i, b := range file.Books {
		divider := b.DividerBefore
		book := models.Book{
			Title:         b.FullName,
			ShortName:     b.Name,
			Number:        i + 1,
			DividerBefore: &divider,
			TranslationId: translation.ID,
		}
		if err := tx.Create(&book).Error; err != nil {
			tx.Rollback()
			return &ImportResult{Error: "книга «" + b.Name + "»: " + err.Error()}
		}
		result.Books++

		for ci, chapterVerses := range b.Content {
			chapter := models.Chapter{Number: ci + 1, BookId: book.ID}
			if err := tx.Create(&chapter).Error; err != nil {
				tx.Rollback()
				return &ImportResult{Error: fmt.Sprintf("%s %d: %s", b.Name, ci+1, err.Error())}
			}
			result.Chapters++

			verses := make([]models.Verse, 0, len(chapterVerses))
			for vi, text := range chapterVerses {
				if text == "" {
					continue // стиха нет в этом переводе — пропуск сохраняет нумерацию
				}
				verses = append(verses, models.Verse{Text: text, Number: vi + 1, ChapterId: chapter.ID})
			}
			if len(verses) == 0 {
				continue
			}
			if err := tx.CreateInBatches(&verses, 500).Error; err != nil {
				tx.Rollback()
				return &ImportResult{Error: fmt.Sprintf("%s %d: %s", b.Name, ci+1, err.Error())}
			}
			result.Verses += len(verses)
		}

		g.emit("import_progress", map[string]any{
			"name":  file.Name,
			"done":  i + 1,
			"total": len(file.Books),
			"book":  b.FullName,
		})
	}

	if err := tx.Commit().Error; err != nil {
		return &ImportResult{Error: "не удалось сохранить: " + err.Error()}
	}

	result.TranslationId = translation.ID
	log.Printf("ImportTranslation: %s — книг %d, глав %d, стихов %d",
		file.Name, result.Books, result.Chapters, result.Verses)

	g.emit("translations_update", nil)

	// Индексация ~6 секунд на перевод — в фоне, чтобы диалог импорта закрылся
	// сразу после записи в базу. До её окончания перевод просто не находится
	// поиском, всё остальное с ним уже работает.
	go g.indexTranslationSilent(translation.ID)

	return result
}

// maxImportFileSize guards against picking a wrong (huge) file by mistake. A
// full Bible export is around 6 MB, so 100 MB is generous.
const maxImportFileSize = 100 << 20

// FilePick is what the UI gets after the operator chose a file: where it lives
// plus what importing it would do. Path is empty when the dialog was cancelled.
type FilePick struct {
	Path     string         `json:"path"`
	FileName string         `json:"fileName"`
	Preview  *ImportPreview `json:"preview"`
}

// PickTranslationFile opens the native file dialog and inspects the chosen
// file. The file itself is read on the Go side — only the path and the summary
// cross the bridge, so a 6 MB translation does not travel through the webview.
func (g *DbHandler) PickTranslationFile() *FilePick {
	if g.app == nil {
		return &FilePick{Preview: &ImportPreview{Error: "окно приложения недоступно"}}
	}

	dialog := g.app.Dialog.OpenFile()
	dialog.SetTitle("Выберите файл перевода")
	dialog.AddFilter("Перевод Библии (JSON)", "*.json")
	dialog.CanChooseFiles(true)

	path, err := dialog.PromptForSingleSelection()
	if err != nil {
		return &FilePick{Preview: &ImportPreview{Error: "не удалось открыть диалог: " + err.Error()}}
	}
	if path == "" {
		return &FilePick{} // отмена — не ошибка
	}

	data, readErr := readImportFile(path)
	if readErr != "" {
		return &FilePick{Path: path, FileName: filepath.Base(path), Preview: &ImportPreview{Error: readErr}}
	}

	return &FilePick{
		Path:     path,
		FileName: filepath.Base(path),
		Preview:  g.InspectTranslationFile(data),
	}
}

// readImportFile reads a translation file off disk, returning a human-readable
// message instead of an error so callers can pass it straight to the UI.
func readImportFile(path string) ([]byte, string) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, "файл не найден"
	}
	if info.Size() > maxImportFileSize {
		return nil, "файл слишком большой (больше 100 МБ)"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "не удалось прочитать файл: " + err.Error()
	}
	return data, ""
}

// ImportTranslationFromFile imports a file the operator picked earlier. name
// and shortName override the file header — the UI sends whatever is in its
// input fields.
func (g *DbHandler) ImportTranslationFromFile(path, name, shortName string) *ImportResult {
	data, readErr := readImportFile(path)
	if readErr != "" {
		return &ImportResult{Error: readErr}
	}
	return g.ImportTranslation(name, shortName, data)
}

// shownVerseState is nil-safe: the broadcaster is only wired up in main(), so
// tests construct a bare DbHandler and would otherwise panic here.
func (g *DbHandler) shownVerseState() *ShownVerse {
	if g.verseB == nil {
		return nil
	}
	return g.verseB.state
}

// ListTranslationSummaries backs the translation list in settings: name, size
// and whether the currently shown verse belongs to it.
func (g *DbHandler) ListTranslationSummaries() []TranslationSummary {
	var translations []models.Translation
	if err := inits.DB.Order("id").Find(&translations).Error; err != nil {
		log.Println("ListTranslationSummaries:", err)
		return nil
	}

	shownTranslationId := uint(0)
	if shown := g.shownVerseState(); shown != nil {
		shownTranslationId = shown.Translation.ID
	}

	out := make([]TranslationSummary, 0, len(translations))
	for _, t := range translations {
		var books, verses int64
		inits.DB.Model(&models.Book{}).Where("translation_id = ?", t.ID).Count(&books)
		inits.DB.Model(&models.Verse{}).
			Where("chapter_id IN (?)",
				inits.DB.Model(&models.Chapter{}).
					Select("chapters.id").
					Joins("JOIN books ON books.id = chapters.book_id").
					Where("books.translation_id = ?", t.ID),
			).Count(&verses)

		out = append(out, TranslationSummary{
			ID:        t.ID,
			Name:      t.Name,
			ShortName: t.ShortName,
			Books:     int(books),
			Verses:    int(verses),
			InUse:     t.ID == shownTranslationId,
		})
	}
	return out
}

// RemoveTranslation deletes a translation with all its books, chapters and
// verses. If the verse currently on the projector came from it, that verse is
// hidden first — otherwise the projector would keep showing text whose rows no
// longer exist.
func (g *DbHandler) RemoveTranslation(idF float32) string {
	id := uint(idF)

	var count int64
	inits.DB.Model(&models.Translation{}).Count(&count)
	if count <= 1 {
		return "нельзя удалить единственный перевод"
	}

	var translation models.Translation
	if err := inits.DB.First(&translation, id).Error; err != nil {
		return "перевод не найден"
	}

	if shown := g.shownVerseState(); shown != nil && shown.Translation.ID == id {
		g.hideVerseInternal()
	}

	// Список стихов снимается до удаления: индекс чистится по идентификаторам,
	// а после каскада их уже не восстановить.
	verseIds := verseIdsOfTranslation(id)

	tx := inits.DB.Begin()
	if tx.Error != nil {
		return "не удалось начать удаление: " + tx.Error.Error()
	}

	bookIds := tx.Model(&models.Book{}).Select("id").Where("translation_id = ?", id)
	chapterIds := tx.Model(&models.Chapter{}).Select("id").Where("book_id IN (?)", bookIds)

	if err := tx.Where("chapter_id IN (?)", chapterIds).Delete(&models.Verse{}).Error; err != nil {
		tx.Rollback()
		return "стихи: " + err.Error()
	}
	if err := tx.Where("book_id IN (?)", bookIds).Delete(&models.Chapter{}).Error; err != nil {
		tx.Rollback()
		return "главы: " + err.Error()
	}
	if err := tx.Where("translation_id = ?", id).Delete(&models.Book{}).Error; err != nil {
		tx.Rollback()
		return "книги: " + err.Error()
	}
	if err := tx.Delete(&models.Translation{}, id).Error; err != nil {
		tx.Rollback()
		return "перевод: " + err.Error()
	}

	if err := tx.Commit().Error; err != nil {
		return "не удалось сохранить удаление: " + err.Error()
	}

	log.Printf("RemoveTranslation: удалён «%s»", translation.Name)
	g.emit("translations_update", nil)
	go g.unindexVersesSilent(verseIds)
	return ""
}
