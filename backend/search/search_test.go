package search

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"unicode/utf16"
)

// sliceUTF16 режет строку так же, как это делает String.prototype.slice в JS —
// по единицам UTF-16. Именно в них Highlight отдаёт свои позиции, и именно так
// они применяются на фронте.
func sliceUTF16(s string, start, length int) string {
	units := utf16.Encode([]rune(s))
	if start < 0 || start+length > len(units) {
		return ""
	}
	return string(utf16.Decode(units[start : start+length]))
}

func TestNormalize(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Любовь", "любовь"},
		{"ШЁЛ", "шел"},
		{"Ёж", "еж"},
		// Разложенная «й» (и + U+0306) — так записан текст Открытой Библии.
		// Без склейки в NFC слово не совпало бы с обычным «мой».
		{"мой", "мой"},
	}
	for _, c := range cases {
		if got := Normalize(c.in); got != c.want {
			t.Errorf("Normalize(%q) = %q, ожидалось %q", c.in, got, c.want)
		}
	}
}

// TestExpandSymmetry — главный тест морфологии. Проверяется именно
// пересечение наборов, а не равенство «лемм»: gomorphy не инвариантен
// относительно входной формы, и вся схема держится на том, что расширение
// применяется симметрично с обеих сторон (см. комментарий к пакету).
func TestExpandSymmetry(t *testing.T) {
	pairs := [][2]string{
		{"любовь", "любви"},   // беглая гласная — на ней ломается лемматизация
		{"человек", "людей"},  // супплетивная форма
		{"идти", "шел"},       // супплетив плюс свёртка ё
		{"господь", "господа"},
		{"молитва", "молитве"},
		{"грех", "грехов"},
		{"возлюбить", "возлюбил"},
		{"мать", "матери"},
	}
	for _, p := range pairs {
		query, text := Expand(Normalize(p[0])), Expand(Normalize(p[1]))
		if !intersects(query, text) {
			t.Errorf("запрос %q не находит текст %q: %v vs %v", p[0], p[1], query, text)
		}
	}
}

// Слова вне словаря (имена, архаизмы) должны деградировать до стемминга, а не
// пропадать: набор всегда непустой и самосовпадающий.
func TestExpandUnknownWords(t *testing.T) {
	for _, w := range []string{"мелхиседек", "рече", "авраам"} {
		got := Expand(w)
		if len(got) == 0 {
			t.Errorf("Expand(%q) пуст", w)
		}
		if !intersects(got, Expand(w)) {
			t.Errorf("Expand(%q) не совпадает сам с собой", w)
		}
	}
}

// Позиции подсветки считаются в единицах UTF-16 и должны корректно резать
// строку на стороне JS — здесь это моделируется тем же срезом.
func TestHighlightUTF16Positions(t *testing.T) {
	text := "Ибо так возлюбил Бог мир, что отдал Сына Своего"
	matches := Highlight(text, QueryTerms("возлюбил мир"))
	if len(matches) != 2 {
		t.Fatalf("ожидалось 2 совпадения, получено %d: %v", len(matches), matches)
	}
	got := []string{}
	for _, m := range matches {
		s := sliceUTF16(text, m.Start, m.Len)
		if s == "" {
			t.Fatalf("совпадение выходит за границы: %+v", m)
		}
		got = append(got, s)
	}
	want := []string{"возлюбил", "мир"}
	if !slices.Equal(got, want) {
		t.Errorf("подсвечено %v, ожидалось %v", got, want)
	}
}

// Текст куплетов пишет оператор, и туда попадают эмодзи. Символ вне BMP стоит
// две единицы UTF-16 при одной руне: считай Highlight в рунах — и всё, что
// стоит после эмодзи, подсветилось бы со сдвигом, а срез пришёлся бы на
// середину суррогатной пары.
func TestHighlightAfterAstralChar(t *testing.T) {
	text := "Хвала 🙌 Господу вовеки"
	matches := Highlight(text, QueryTerms("господь"))
	if len(matches) != 1 {
		t.Fatalf("ожидалось 1 совпадение, получено %d: %v", len(matches), matches)
	}
	if got := sliceUTF16(text, matches[0].Start, matches[0].Len); got != "Господу" {
		t.Errorf("подсвечено %q, ожидалось \"Господу\"", got)
	}
}

// Подсветка обязана согласовываться с морфологией поиска: если запрос нашёл
// стих по другой словоформе, подсветить надо именно её, иначе оператор видит
// результат без единого выделенного слова.
func TestHighlightMatchesOtherForm(t *testing.T) {
	matches := Highlight("Так говорит Господь людям Своим", QueryTerms("человек"))
	if len(matches) == 0 {
		t.Fatal("морфологическое совпадение не подсвечено")
	}
	if got := sliceUTF16("Так говорит Господь людям Своим", matches[0].Start, matches[0].Len); got != "людям" {
		t.Errorf("подсвечено %q, ожидалось \"людям\"", got)
	}
}

// Известное ограничение: словарь OpenCorpora описывает современный русский, а
// Синодальный перевод полон церковнославянских форм («труждающиеся», «рече»,
// «глаголющий»). Они не связаны с современными словами ни одной парадигмой,
// поэтому находятся только по себе самим. Тест фиксирует это как осознанную
// границу возможностей, а не как дефект: лечится словарём синонимов, которого
// в системе намеренно нет.
func TestArchaicFormsAreNotLinked(t *testing.T) {
	if len(Highlight("все труждающиеся", QueryTerms("трудиться"))) != 0 {
		t.Log("архаичная форма неожиданно связалась с современной — ограничение снято, тест можно убрать")
	}
	if len(Highlight("все труждающиеся", QueryTerms("труждающиеся"))) == 0 {
		t.Error("архаичная форма не находится даже по самой себе")
	}
}

func TestFixLayout(t *testing.T) {
	if got := FixLayout("k.,jdm"); got != "любовь" {
		t.Errorf("FixLayout(\"k.,jdm\") = %q, ожидалось \"любовь\"", got)
	}
	if HasLatin("любовь") {
		t.Error("HasLatin ложно срабатывает на кириллице")
	}
	if !HasLatin("k.,jdm") {
		t.Error("HasLatin не видит латиницу")
	}
}

// Сквозная проверка индекса: документ находится по другой словоформе, по
// опечатке и по забытой раскладке, а фильтр по переводу не пропускает чужое.
func TestIndexSearch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "idx.bleve")
	idx, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() {
		idx.Close()
		os.RemoveAll(dir)
	}()

	err = idx.PutBatch([]Doc{
		{ID: 1, Kind: KindVerse, TranslationID: 1, Text: "Ибо так возлюбил Бог мир, что отдал Сына Своего Единородного"},
		{ID: 2, Kind: KindVerse, TranslationID: 1, Text: "В начале сотворил Бог небо и землю"},
		{ID: 3, Kind: KindVerse, TranslationID: 2, Text: "Ибо так возлюбил Бог мир — другой перевод"},
		{ID: 4, Kind: KindCouplet, Text: "Великий Бог, когда на мир смотрю я"},
	})
	if err != nil {
		t.Fatalf("PutBatch: %v", err)
	}

	find := func(q string, o Opts) []Hit {
		t.Helper()
		hits, _, err := idx.Search(q, o)
		if err != nil {
			t.Fatalf("Search(%q): %v", q, err)
		}
		return hits
	}

	verses := Opts{Kind: KindVerse, TranslationID: 1, Limit: 10, Fuzzy: true}

	if hits := find("возлюбить мир", verses); len(hits) != 1 || hits[0].ID != 1 {
		t.Errorf("поиск по другой словоформе дал %v", hits)
	}
	if hits := find("вазлюбил", verses); len(hits) == 0 {
		t.Error("опечатка не исправлена нечётким поиском")
	}
	if hits := find("djpk.,bk", verses); len(hits) != 1 || hits[0].ID != 1 {
		t.Errorf("забытая раскладка не распознана: %v", hits)
	}
	if hits := find("сотворил", verses); len(hits) != 1 || hits[0].ID != 2 {
		t.Errorf("ожидался стих 2, получено %v", hits)
	}
	// Стих 3 лежит в другом переводе и не должен всплыть.
	for _, h := range find("возлюбил", verses) {
		if h.ID == 3 {
			t.Error("фильтр по переводу пропустил чужой стих")
		}
	}
	// Куплеты живут в том же индексе, но не должны попадать в выдачу стихов.
	for _, h := range find("мир", verses) {
		if h.ID == 4 {
			t.Error("куплет попал в выдачу стихов")
		}
	}
	if hits := find("смотреть", Opts{Kind: KindCouplet, Limit: 10}); len(hits) != 1 || hits[0].ID != 4 {
		t.Errorf("поиск по куплетам дал %v", hits)
	}
}

// Точное совпадение обязано опережать морфологическое: в парадигму глагола по
// OpenCorpora входят причастия, поэтому без буста «возлюбил» уравнивается с
// «возлюбленному» и оператор получает не тот стих.
func TestExactFormRanksHigher(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "rank.bleve")
	idx, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() {
		idx.Close()
		os.RemoveAll(dir)
	}()

	err = idx.PutBatch([]Doc{
		{ID: 1, Kind: KindVerse, TranslationID: 1, Text: "Тимофею, возлюбленному сыну: благодать, милость, мир от Бога Отца"},
		{ID: 2, Kind: KindVerse, TranslationID: 1, Text: "Ибо так возлюбил Бог мир, что отдал Сына Своего Единородного"},
	})
	if err != nil {
		t.Fatalf("PutBatch: %v", err)
	}

	hits, _, err := idx.Search("возлюбил мир", Opts{Kind: KindVerse, TranslationID: 1, Limit: 10})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) < 2 {
		t.Fatalf("ожидалось два попадания, получено %d", len(hits))
	}
	if hits[0].ID != 2 {
		t.Errorf("первым идёт документ %d, ожидался 2 (дословное совпадение)", hits[0].ID)
	}
}

func TestDeleteRemovesFromIndex(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "del.bleve")
	idx, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() {
		idx.Close()
		os.RemoveAll(dir)
	}()

	if err := idx.Put(Doc{ID: 7, Kind: KindCouplet, Text: "Тихая ночь, дивная ночь"}); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if hits, _, _ := idx.Search("ночь", Opts{Kind: KindCouplet, Limit: 5}); len(hits) != 1 {
		t.Fatalf("куплет не проиндексирован")
	}
	if err := idx.Delete(KindCouplet, 7); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if hits, _, _ := idx.Search("ночь", Opts{Kind: KindCouplet, Limit: 5}); len(hits) != 0 {
		t.Error("удалённый куплет всё ещё находится")
	}
}

func intersects(a, b []string) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}
