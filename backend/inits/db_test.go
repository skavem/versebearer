package inits

import "testing"

func TestVersionLT(t *testing.T) {
	cases := []struct {
		v    string
		n    int
		want bool
	}{
		{"", 4, true}, // пустая строка — свежая БД, должна пройти все блоки
		{"6", 7, true},
		{"10", 4, false}, // главный кейс бага: раньше "10" < "4" было true при строковом сравнении
		{"abc", 1, true}, // нечисловой мусор трактуется как 0
		{"7", 7, false},
	}
	for _, c := range cases {
		if got := versionLT(c.v, c.n); got != c.want {
			t.Errorf("versionLT(%q, %d) = %v, want %v", c.v, c.n, got, c.want)
		}
	}
}
