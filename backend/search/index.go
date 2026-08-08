package search

import (
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	index "github.com/blevesearch/bleve_index_api"

	// Анализаторы в Bleve подключаются регистрацией через побочный эффект
	// импорта — без этих двух строк маппинг падает на «no analyzer with name
	// 'simple' registered» уже при создании индекса.
	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/keyword"
	_ "github.com/blevesearch/bleve/v2/analysis/analyzer/simple"
)

// Поля документа в индексе.
//
// morph хранит стеммы словоформ (см. ExpandText) — по нему идёт основной
// поиск. text хранит нормализованный оригинал — по нему ищется точная фраза и
// считается буст, чтобы дословное совпадение всплывало над морфологическим.
// Оба поля не stored: сам текст для выдачи берётся из SQLite по идентификатору,
// дублировать его в индексе незачем.
const (
	fieldKind        = "kind"
	fieldTranslation = "tr"
	fieldMorph       = "morph"
	fieldText        = "text"
)

// Типы документов в индексе. Индекс общий для стихов и куплетов: запросы почти
// всегда идут в одну из областей, но общий индекс даёт согласованное
// ранжирование, если однажды понадобится искать по обеим сразу.
const (
	KindVerse   = "verse"
	KindCouplet = "couplet"
)

// Doc — то, что кладётся в индекс. Text — исходный текст стиха или куплета,
// расширение в стеммы делается внутри Index/IndexBatch.
type Doc struct {
	ID            uint
	Kind          string
	TranslationID uint // только для стихов; для куплетов 0
	Text          string
}

// docID собирает ключ документа. Стихи и куплеты нумеруются независимо, так
// что тип обязан входить в ключ.
func docID(kind string, id uint) string {
	return fmt.Sprintf("%s:%d", kind, id)
}

// Index — обёртка над Bleve. Все методы безопасны для конкурентного вызова:
// Bleve потокобезопасен сам, mu защищает только подмену индекса при полной
// перестройке.
type Index struct {
	mu   sync.RWMutex
	idx  bleve.Index
	path string
}

// buildMapping описывает единственный тип документа.
//
// Анализатор "simple" (юникодный разбор на слова + нижний регистр) годится для
// обоих текстовых полей: в morph лежат уже готовые стеммы через пробел, в text
// — уже прогнанный через Normalize текст. Вся тяжёлая работа сделана до
// индексации, поэтому языковых анализаторов Bleve здесь не требуется.
func buildMapping() mapping.IndexMapping {
	textField := bleve.NewTextFieldMapping()
	textField.Analyzer = "simple"
	textField.Store = false
	textField.IncludeInAll = false
	textField.IncludeTermVectors = false

	kindField := bleve.NewKeywordFieldMapping()
	kindField.Store = false
	kindField.IncludeInAll = false

	doc := bleve.NewDocumentMapping()
	doc.AddFieldMappingsAt(fieldMorph, textField)
	doc.AddFieldMappingsAt(fieldText, textField)
	doc.AddFieldMappingsAt(fieldKind, kindField)
	doc.AddFieldMappingsAt(fieldTranslation, kindField)

	m := bleve.NewIndexMapping()
	m.DefaultMapping = doc
	m.DefaultAnalyzer = "simple"
	// BM25 вместо устаревшего tf-idf: он нормирует на длину документа, а стихи
	// различаются по длине в разы — без нормировки короткие стихи получали бы
	// незаслуженное преимущество.
	m.ScoringModel = index.BM25Scoring
	return m
}

// Open открывает индекс по пути, создавая его при отсутствии.
func Open(path string) (*Index, error) {
	idx, err := bleve.Open(path)
	if errors.Is(err, bleve.ErrorIndexPathDoesNotExist) {
		idx, err = bleve.New(path, buildMapping())
	}
	if err != nil {
		return nil, fmt.Errorf("открыть поисковый индекс: %w", err)
	}
	return &Index{idx: idx, path: path}, nil
}

// Close закрывает индекс.
func (i *Index) Close() error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.idx == nil {
		return nil
	}
	err := i.idx.Close()
	i.idx = nil
	return err
}

// Count возвращает число документов в индексе — по нему на старте решается,
// нужна ли первичная сборка.
func (i *Index) Count() (uint64, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.idx == nil {
		return 0, errors.New("индекс закрыт")
	}
	return i.idx.DocCount()
}

// Put добавляет или заменяет один документ. Используется для точечных правок
// куплетов — переиндексировать всё ради одного изменённого блока незачем.
func (i *Index) Put(d Doc) error {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.idx == nil {
		return errors.New("индекс закрыт")
	}
	return i.idx.Index(docID(d.Kind, d.ID), fields(d))
}

// Delete убирает документ из индекса.
func (i *Index) Delete(kind string, id uint) error {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.idx == nil {
		return errors.New("индекс закрыт")
	}
	return i.idx.Delete(docID(kind, id))
}

// PutBatch индексирует пачку документов. Bleve заметно быстрее работает
// батчами, чем поштучно, поэтому массовая индексация идёт только через него.
func (i *Index) PutBatch(docs []Doc) error {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.idx == nil {
		return errors.New("индекс закрыт")
	}
	batch := i.idx.NewBatch()
	for _, d := range docs {
		if err := batch.Index(docID(d.Kind, d.ID), fields(d)); err != nil {
			return err
		}
	}
	return i.idx.Batch(batch)
}

// fields раскладывает документ по полям индекса. Расширение в стеммы (самая
// дорогая часть, ~5 мкс на слово) происходит именно здесь.
func fields(d Doc) map[string]any {
	f := map[string]any{
		fieldKind:  d.Kind,
		fieldMorph: ExpandText(d.Text),
		fieldText:  Normalize(d.Text),
	}
	if d.TranslationID != 0 {
		f[fieldTranslation] = fmt.Sprintf("%d", d.TranslationID)
	}
	return f
}

// Reset удаляет индекс с диска и создаёт пустой. Нужен для полной пересборки:
// удалять документы по одному дороже и оставляет мусор в сегментах.
func (i *Index) Reset() error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if i.idx != nil {
		if err := i.idx.Close(); err != nil {
			return err
		}
		i.idx = nil
	}
	if err := os.RemoveAll(i.path); err != nil {
		return fmt.Errorf("удалить старый индекс: %w", err)
	}
	idx, err := bleve.New(i.path, buildMapping())
	if err != nil {
		return fmt.Errorf("пересоздать индекс: %w", err)
	}
	i.idx = idx
	return nil
}
