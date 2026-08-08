package main

import (
	"regexp"
	"strconv"
	"strings"

	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/search"
)

// Разбор ссылки на стих: «Ин 3:16», «1 Кор 13», «быт1:1», «иоанна 3 16».
//
// Это отдельный от полнотекстового поиска путь и в живом служении основной:
// оператор чаще знает, куда идти, чем ищет по словам. Поэтому ссылка
// разбирается до поиска и, если разобралась, показывается первой строкой.

// referenceRe разбирает строку на три части: название книги (с возможным
// ведущим номером — «1 Кор»), номер главы и необязательный номер стиха.
// Разделителем между главой и стихом служит двоеточие, точка или пробел.
var referenceRe = regexp.MustCompile(`^\s*(\d?\s*[\p{L}]+\.?)\s*(\d+)(?:\s*[:.\s]\s*(\d+))?\s*$`)

// ReferenceHit — разобранная ссылка. Chapter и Verse могут быть нулевыми, если
// в книге нет такой главы или стиха: тогда переход всё равно возможен, но на
// то, что нашлось.
type ReferenceHit struct {
	Book    models.Book    `json:"book"`
	Chapter models.Chapter `json:"chapter"`
	Verse   models.Verse   `json:"verse"`
	// Ref — то, как ссылку показать оператору: «Ин 3:16».
	Ref string `json:"ref"`
	// HasVerse отличает «Ин 3» (перейти к главе) от «Ин 3:16» (выделить стих).
	HasVerse bool `json:"hasVerse"`
}

// ParseReference пытается прочитать строку как ссылку на стих в указанном
// переводе. Возвращает nil, если строка на ссылку не похожа или книга не
// нашлась — тогда UI показывает только результаты полнотекстового поиска.
func (g *DbHandler) ParseReference(query string, translationId float32) *ReferenceHit {
	m := referenceRe.FindStringSubmatch(query)
	if m == nil {
		return nil
	}

	chapterNum, err := strconv.Atoi(m[2])
	if err != nil {
		return nil
	}
	verseNum := 0
	if m[3] != "" {
		verseNum, _ = strconv.Atoi(m[3])
	}

	var books []models.Book
	if err := inits.DB.Where("translation_id = ?", uint(translationId)).Order("number").Find(&books).Error; err != nil {
		return nil
	}
	book := matchBook(m[1], books)
	if book == nil {
		return nil
	}

	hit := &ReferenceHit{Book: *book, Ref: book.ShortName + " " + m[2]}

	var chapter models.Chapter
	if err := inits.DB.Where("book_id = ? AND number = ?", book.ID, chapterNum).First(&chapter).Error; err != nil {
		return nil // главы нет — ссылка бессмысленна
	}
	hit.Chapter = chapter

	if verseNum > 0 {
		var verse models.Verse
		if err := inits.DB.Where("chapter_id = ? AND number = ?", chapter.ID, verseNum).First(&verse).Error; err == nil {
			hit.Verse = verse
			hit.HasVerse = true
			hit.Ref = book.ShortName + " " + m[2] + ":" + m[3]
		}
	}
	return hit
}

// matchBook подбирает книгу по началу названия. Порядок проверок — от самого
// надёжного совпадения к самому расплывчатому, чтобы «ин» уходило в Иоанна, а
// не в Иону только из-за порядка книг в базе.
func matchBook(raw string, books []models.Book) *models.Book {
	needle := normalizeBookName(raw)
	if needle == "" {
		return nil
	}

	// Забытая раскладка работает и здесь: «BY» вместо «Ин».
	candidates := []string{needle}
	if search.HasLatin(raw) {
		candidates = append(candidates, normalizeBookName(search.FixLayout(raw)))
	}

	for _, needle := range candidates {
		for _, matches := range []func(b models.Book) bool{
			func(b models.Book) bool { return normalizeBookName(b.ShortName) == needle },
			func(b models.Book) bool { return normalizeBookName(b.Title) == needle },
			func(b models.Book) bool { return strings.HasPrefix(normalizeBookName(b.ShortName), needle) },
			func(b models.Book) bool { return strings.HasPrefix(normalizeBookName(b.Title), needle) },
		} {
			for i := range books {
				if matches(books[i]) {
					return &books[i]
				}
			}
		}
	}
	return nil
}

// normalizeBookName убирает всё, что оператор может набрать или пропустить не
// глядя: регистр, ё, точки и пробелы внутри («1 Кор.» == «1кор»).
func normalizeBookName(s string) string {
	s = search.Normalize(s)
	s = strings.ReplaceAll(s, " ", "")
	return strings.ReplaceAll(s, ".", "")
}
