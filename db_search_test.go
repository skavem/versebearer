package main

import (
	"path/filepath"
	"testing"

	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/search"
)

// withSearchIndex поднимает временный индекс на обработчике. Путь берётся из
// TempDir, а не из searchIndexPath, чтобы тест не трогал рабочий индекс рядом
// с test.db.
func withSearchIndex(t *testing.T, g *DbHandler) {
	t.Helper()
	idx, err := search.Open(filepath.Join(t.TempDir(), "idx.bleve"))
	if err != nil {
		t.Fatalf("open index: %v", err)
	}
	t.Cleanup(func() { idx.Close() })
	g.searchIdx = idx
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
