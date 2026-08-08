package search

import "strings"

// Раскладка ЙЦУКЕН поверх QWERTY: что печатается, если забыть переключить
// язык. Оператор во время служения смотрит на экран, а не на клавиатуру, и
// «k.,jdm» вместо «любовь» — рядовая ситуация, а не редкость.
var qwertyToRu = map[rune]rune{
	'q': 'й', 'w': 'ц', 'e': 'у', 'r': 'к', 't': 'е', 'y': 'н', 'u': 'г',
	'i': 'ш', 'o': 'щ', 'p': 'з', '[': 'х', ']': 'ъ',
	'a': 'ф', 's': 'ы', 'd': 'в', 'f': 'а', 'g': 'п', 'h': 'р', 'j': 'о',
	'k': 'л', 'l': 'д', ';': 'ж', '\'': 'э',
	'z': 'я', 'x': 'ч', 'c': 'с', 'v': 'м', 'b': 'и', 'n': 'т', 'm': 'ь',
	',': 'б', '.': 'ю', '/': '.', '`': 'ё',
}

// HasLatin сообщает, есть ли в строке латиница — признак того, что стоит
// попробовать её перевести в кириллицу.
func HasLatin(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return true
		}
	}
	return false
}

// FixLayout переводит строку так, как если бы её набрали в русской раскладке.
// Символы, которым нет соответствия, остаются как есть.
func FixLayout(s string) string {
	var b strings.Builder
	b.Grow(len(s) * 2)
	for _, r := range strings.ToLower(s) {
		if ru, ok := qwertyToRu[r]; ok {
			b.WriteRune(ru)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
