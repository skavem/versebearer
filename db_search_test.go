package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/search"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// withSearchIndex поднимает временный индекс на обработчике. Путь берётся из
// TempDir, а не из paths.SearchIndexPath(), чтобы тест не трогал рабочий
// индекс оператора.
func withSearchIndex(t *testing.T, g *DbHandler) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "idx.bleve")
	idx, err := search.Open(path)
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	g.searchIdx = idx
	// searchIdxPath — иначе RebuildSearchIndex не найдёт куда писать маркер
	// завершённой сборки и (безопасно) пропустит его, но тест тогда не
	// проверял бы реальный путь выполнения.
	g.searchIdxPath = path
}

// seedBible создаёт минимальный перевод: одна книга, одна глава, два стиха.
func seedBible(t *testing.T) uint {
	t.Helper()
	translation := &models.Translation{Name: "Синодальный", ShortName: "SND"}
	if err := inits.DB.Create(translation).Error; err != nil {
		t.Fatal(err)
	}
	book := &models.Book{Title: "Иоанна", ShortName: "Ин", Number: 43, TranslationId: translation.ID}
	if err := inits.DB.Create(book).Error; err != nil {
		t.Fatal(err)
	}
	chapter := &models.Chapter{Number: 3, BookId: book.ID}
	if err := inits.DB.Create(chapter).Error; err != nil {
		t.Fatal(err)
	}
	verses := []models.Verse{
		{Number: 16, ChapterId: chapter.ID, Text: "Ибо так возлюбил Бог мир, что отдал Сына Своего Единородного"},
		{Number: 17, ChapterId: chapter.ID, Text: "Ибо не послал Бог Сына Своего в мир, чтобы судить мир"},
	}
	for i := range verses {
		if err := inits.DB.Create(&verses[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	return translation.ID
}

// Сквозная проверка сборки индекса поверх настоящих GORM-запросов: join
// verses→chapters→books живёт только здесь и в рантайме, юнит-тесты пакета
// search его не касаются.
func TestRebuildSearchIndexAndSearch(t *testing.T) {
	setupTestDB(t)
	g := &DbHandler{}
	withSearchIndex(t, g)

	translationId := seedBible(t)
	seedTwoSongs(t)

	if err := g.RebuildSearchIndex(); err != nil {
		t.Fatalf("RebuildSearchIndex: %v", err)
	}

	// Запрос начальной формой должен найти стих, где стоит другая форма.
	hits := g.SearchVerses("возлюбить", float32(translationId), 10)
	if len(hits) != 1 {
		t.Fatalf("ожидался один стих, получено %d", len(hits))
	}
	hit := hits[0]
	if hit.Verse.Number != 16 {
		t.Errorf("найден стих %d, ожидался 16", hit.Verse.Number)
	}
	// Связи обязаны быть заполнены — фронт передаёт результат прямо в переход.
	if hit.Book.ShortName != "Ин" || hit.Chapter.Number != 3 || hit.Translation.ID != translationId {
		t.Errorf("связи не заполнены: книга=%q глава=%d перевод=%d",
			hit.Book.ShortName, hit.Chapter.Number, hit.Translation.ID)
	}
	if len(hit.Matches) == 0 {
		t.Error("совпадения для подсветки не вернулись")
	}

	// Пустой запрос не должен возвращать всю Библию.
	if got := g.SearchVerses("", float32(translationId), 10); len(got) != 0 {
		t.Errorf("пустой запрос вернул %d результатов", len(got))
	}
}

// Хуки синхронизации: созданный куплет должен находиться сразу, удалённый —
// исчезать. Именно этот путь ломается тише всего, если забыть вызов в новой
// мутации.
func TestCoupletIndexHooks(t *testing.T) {
	setupTestDB(t)
	g := &DbHandler{}
	withSearchIndex(t, g)

	songA, _ := seedTwoSongs(t)

	g.CreateCouplet("Великий Бог, когда на мир смотрю я", "Куплет 1", 1, songA)

	hits := g.SearchCouplets("смотреть", 10)
	if len(hits) != 1 {
		t.Fatalf("созданный куплет не найден: %d результатов", len(hits))
	}
	if hits[0].Song.ID != songA {
		t.Errorf("куплет привязан к песне %d, ожидалась %d", hits[0].Song.ID, songA)
	}
	created := hits[0].Couplet.ID

	g.UpdateCouplet(int(created), "Куплет 1", "Тихая ночь, дивная ночь", 1)
	if got := g.SearchCouplets("смотреть", 10); len(got) != 0 {
		t.Error("после правки куплет всё ещё находится по старому тексту")
	}
	if got := g.SearchCouplets("ночь", 10); len(got) != 1 {
		t.Errorf("после правки куплет не находится по новому тексту: %d", len(got))
	}

	g.RemoveCouplet(int(created))
	if got := g.SearchCouplets("ночь", 10); len(got) != 0 {
		t.Error("удалённый куплет всё ещё находится")
	}
}

func TestParseReference(t *testing.T) {
	setupTestDB(t)
	g := &DbHandler{}
	translationId := seedBible(t)

	cases := []struct {
		query    string
		wantRef  string
		hasVerse bool
	}{
		{"Ин 3:16", "Ин 3:16", true},
		{"ин3:16", "Ин 3:16", true},
		{"иоанна 3 16", "Ин 3:16", true},
		{"Ин 3", "Ин 3", false},
		{"BY 3:16", "Ин 3:16", true}, // забытая раскладка
	}
	for _, c := range cases {
		got := g.ParseReference(c.query, float32(translationId))
		if got == nil {
			t.Errorf("%q не разобрался", c.query)
			continue
		}
		if got.Ref != c.wantRef || got.HasVerse != c.hasVerse {
			t.Errorf("%q → %q (стих: %v), ожидалось %q (стих: %v)",
				c.query, got.Ref, got.HasVerse, c.wantRef, c.hasVerse)
		}
	}

	// Обычный текстовый запрос ссылкой быть не должен, иначе поверх выдачи
	// повиснет бессмысленная кнопка перехода.
	if got := g.ParseReference("возлюбил мир", float32(translationId)); got != nil {
		t.Errorf("текстовый запрос разобрался как ссылка: %+v", got)
	}
	// Несуществующая глава — тоже не ссылка.
	if got := g.ParseReference("Ин 99:1", float32(translationId)); got != nil {
		t.Errorf("несуществующая глава разобралась: %+v", got)
	}
}

// TestRebuildSearchIndexMarkerNotRestoredOnFailure — LOW-раздел «Новые
// тесты» обзора: маркер завершённой сборки (searchIndexCompleteMarker)
// обязан сниматься ДО Reset() и писаться только ПОСЛЕ успеха — у механики
// раньше не было ни одного сторожа.
func TestRebuildSearchIndexMarkerNotRestoredOnFailure(t *testing.T) {
	// Намеренно НЕ setupTestDB: пустая, немигрированная база даёт надёжный,
	// портируемый отказ — Reset() успешно отрабатывает на чистом индексе
	// (созданном withSearchIndex мгновением раньше, ничто ему не мешает), а
	// первый же запрос к базе внутри RebuildSearchIndex
	// (inits.DB.Find(&translations)) падает на "no such table: translations".
	// Заставить сорваться сам Reset() портируемо не удалось: единственный
	// проверенный на практике способ (подменить родительский каталог
	// индекса файлом) уничтожает и сам маркер как побочный эффект — маркер
	// лежит рядом с каталогом индекса, в том же родителе, так что тест
	// перестаёт что-либо доказывать. Этот сценарий проверяет то же самое
	// важное свойство с другой стороны: маркер не переживает сорвавшуюся
	// пересборку и не пишется заново, если она не дошла до конца.
	bareDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open bare db: %v", err)
	}
	inits.DB = bareDB

	g := &DbHandler{}
	withSearchIndex(t, g)
	marker := searchIndexCompleteMarker(g.searchIdxPath)

	if err := os.WriteFile(marker, []byte{}, 0o644); err != nil {
		t.Fatalf("seed marker: %v", err)
	}

	if err := g.RebuildSearchIndex(); err == nil {
		t.Fatal("RebuildSearchIndex с немигрированной базой = nil, want error")
	}

	// Маркер, существовавший до вызова, не пережил отказ — снят до того,
	// как стало известно, что сборка сорвётся, и не восстановлен задним
	// числом.
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("маркер пережил сорвавшуюся пересборку: stat err = %v", err)
	}
}

// TestRebuildSearchIndexMarkerWrittenOnlyAfterSuccess — вторая половина той
// же механики: маркер появляется ровно после успешной пересборки, не раньше.
func TestRebuildSearchIndexMarkerWrittenOnlyAfterSuccess(t *testing.T) {
	setupTestDB(t)
	g := &DbHandler{}
	withSearchIndex(t, g)
	marker := searchIndexCompleteMarker(g.searchIdxPath)

	seedBible(t)
	seedTwoSongs(t)

	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("маркер существует до первой сборки: stat err = %v", err)
	}

	if err := g.RebuildSearchIndex(); err != nil {
		t.Fatalf("RebuildSearchIndex: %v", err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("маркер не записан после успешной сборки: %v", err)
	}
}

// TestOpenSearchIndexRebuildsWhenMarkerMissing — маркер отсутствует →
// openSearchIndex запускает фоновую сборку сам, без явного вызова
// RebuildSearchIndex. Единственный тест в пакете, вызывающий настоящий
// (g *DbHandler) openSearchIndex() — он ходит в paths.SearchIndexPath(),
// то есть в единственный на весь процесс теста путь, заданный TestMain
// (см. audio_import_test.go) через VERSEBEARER_DATA; больше ни один тест
// этот путь не трогает, так что открыть его здесь безопасно — конфликтов
// за блокировку индекса Bleve с другим тестом нет.
func TestOpenSearchIndexRebuildsWhenMarkerMissing(t *testing.T) {
	setupTestDB(t)
	seedBible(t)
	seedTwoSongs(t)

	g := &DbHandler{}
	if err := g.openSearchIndex(); err != nil {
		t.Fatalf("openSearchIndex: %v", err)
	}
	t.Cleanup(func() {
		if g.searchIdx != nil {
			g.searchIdx.Close()
		}
	})

	marker := searchIndexCompleteMarker(g.searchIdxPath)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(marker); err == nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("маркер %s не появился за 5с — фоновая сборка не запустилась при отсутствии маркера", marker)
}
