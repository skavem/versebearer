package search

import (
	"errors"
	"strconv"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/search/query"
)

// Hit — попадание: тип, идентификатор записи в SQLite и оценка BM25.
// Сам текст не возвращается — его достаёт вызывающий слой из базы, где он
// лежит вместе со всеми связями (книга, глава, песня).
type Hit struct {
	Kind  string
	ID    uint
	Score float64
}

// Opts — параметры поиска.
type Opts struct {
	Kind          string // KindVerse или KindCouplet
	TranslationID uint   // 0 — не ограничивать переводом (для куплетов всегда 0)
	Limit         int
	Fuzzy         bool // разрешить поиск с опечатками, если точный ничего не дал
}

// Бусты за дословность. Морфологическое совпадение остаётся основным
// механизмом отбора, но само по себе оно слишком «щедрое»: в парадигму глагола
// по OpenCorpora входят и причастия, поэтому «возлюбил» морфологически равен
// «возлюбленному», а запрос «возлюбил мир» без бустов поднимает наверх
// «Тимофею, возлюбленному сыну… мир от Бога» вместо Ин 3:16.
//
// exactBoost вознаграждает документ за каждое слово запроса, встреченное
// дословно; phraseBoost — за все слова подряд. Оба применяются как Should, то
// есть на отбор не влияют, только на порядок.
const (
	exactBoost  = 2.5
	phraseBoost = 4.0
)

// maxLimit — потолок размера выдачи, см. Search.
const maxLimit = 200

// Search выполняет запрос. Возвращает попадания в порядке убывания оценки и
// раскрытые термы запроса — они нужны вызывающему слою для подсветки, чтобы не
// раскрывать запрос второй раз.
func (i *Index) Search(q string, o Opts) ([]Hit, [][]string, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.idx == nil {
		return nil, nil, errors.New("индекс закрыт")
	}

	if o.Limit <= 0 {
		o.Limit = 50
	}
	// Потолок: выдача уходит в SQLite одним `id IN (...)`, и слишком щедрый
	// вызывающий заставил бы Bleve материализовать тысячи попаданий ради списка,
	// который оператор всё равно не пролистает.
	if o.Limit > maxLimit {
		o.Limit = maxLimit
	}

	// Забытая раскладка проверяется раньше опечаток: «k.,jdm» даёт ноль
	// результатов гарантированно, тогда как нечёткий поиск по латинице
	// перебирал бы словарь впустую.
	if HasLatin(q) {
		if fixed := FixLayout(q); fixed != q {
			if hits, terms, err := i.attempt(fixed, o); err != nil || len(hits) > 0 {
				return hits, terms, err
			}
		}
	}

	hits, terms, err := i.attempt(q, o)
	if err != nil || len(hits) > 0 {
		return hits, terms, err
	}

	// Опечатка распознаётся по пустой выдаче: гонять нечёткий поиск всегда
	// дорого и вредно — он размывает нормальные результаты.
	//
	// Термы при этом возвращаются исходные, не размытые, поэтому подсветка на
	// таких результатах не сработает: в найденном тексте стоит другое слово, чем
	// в запросе. Строка находится — это главное; закрасить в ней «не то» слово
	// было бы хуже, чем не закрашивать ничего.
	if o.Fuzzy && len(terms) > 0 {
		hits, err = i.run(buildQuery(q, terms, o, true), o.Limit)
		if err != nil {
			return nil, nil, err
		}
	}
	return hits, terms, nil
}

// attempt выполняет один точный заход по готовой строке запроса.
func (i *Index) attempt(q string, o Opts) ([]Hit, [][]string, error) {
	terms := QueryTerms(q)
	if len(terms) == 0 {
		return nil, nil, nil
	}
	hits, err := i.run(buildQuery(q, terms, o, false), o.Limit)
	return hits, terms, err
}

// buildQuery собирает запрос: все слова обязательны (конъюнкция), внутри слова
// годится любая из его стемм (дизъюнкция), плюс необязательный бонус за
// дословную фразу и обязательные фильтры по типу и переводу.
func buildQuery(raw string, terms [][]string, o Opts, fuzzy bool) query.Query {
	root := bleve.NewBooleanQuery()

	kindQ := bleve.NewTermQuery(o.Kind)
	kindQ.SetField(fieldKind)
	root.AddMust(kindQ)

	if o.TranslationID != 0 {
		trQ := bleve.NewTermQuery(translationTerm(o.TranslationID))
		trQ.SetField(fieldTranslation)
		root.AddMust(trQ)
	}

	words := bleve.NewConjunctionQuery()
	for _, stems := range terms {
		variants := bleve.NewDisjunctionQuery()
		for _, stem := range stems {
			if fuzzy {
				fq := bleve.NewFuzzyQuery(stem)
				fq.SetField(fieldMorph)
				fq.SetFuzziness(1)
				variants.AddQuery(fq)
				continue
			}
			tq := bleve.NewTermQuery(stem)
			tq.SetField(fieldMorph)
			variants.AddQuery(tq)
		}
		words.AddQuery(variants)
	}
	root.AddMust(words)

	// Бонус за каждое дословно встреченное слово запроса. Ищется по полю с
	// оригиналом, поэтому берутся слова исходной строки, а не стеммы.
	for _, word := range wordRe.FindAllString(Normalize(raw), -1) {
		tq := bleve.NewTermQuery(word)
		tq.SetField(fieldText)
		tq.SetBoost(exactBoost)
		root.AddShould(tq)
	}

	// Фразовый буст — за все слова подряд. Одно слово фразой не бывает.
	if len(terms) > 1 {
		pq := bleve.NewMatchPhraseQuery(Normalize(raw))
		pq.SetField(fieldText)
		pq.SetBoost(phraseBoost)
		root.AddShould(pq)
	}

	return root
}

// run выполняет готовый запрос и раскладывает результат.
func (i *Index) run(q query.Query, limit int) ([]Hit, error) {
	req := bleve.NewSearchRequestOptions(q, limit, 0, false)
	res, err := i.idx.Search(req)
	if err != nil {
		return nil, err
	}
	out := make([]Hit, 0, len(res.Hits))
	for _, h := range res.Hits {
		kind, id, ok := parseDocID(h.ID)
		if !ok {
			continue
		}
		out = append(out, Hit{Kind: kind, ID: id, Score: h.Score})
	}
	return out, nil
}

// parseDocID разбирает ключ документа обратно в тип и идентификатор.
func parseDocID(s string) (string, uint, bool) {
	kind, rest, found := strings.Cut(s, ":")
	if !found {
		return "", 0, false
	}
	id, err := strconv.ParseUint(rest, 10, 64)
	if err != nil {
		return "", 0, false
	}
	return kind, uint(id), true
}
