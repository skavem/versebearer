package main

import (
	"fmt"
	"log"

	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/search"
)

// searchIndexPath — каталог индекса Bleve рядом с test.db (см. backend/inits).
// Индекс полностью производен от SQLite: его можно удалить в любой момент,
// на следующем старте он соберётся заново.
const searchIndexPath = "search.bleve"

// indexBatchSize — сколько документов уходит в Bleve за раз. Пачками индексация
// идёт кратно быстрее поштучной, а 1000 стихов — это ещё некрупный кусок
// памяти.
const indexBatchSize = 1000

// VerseSearchHit — найденный стих. Встраивает ShownVerse, поэтому фронт может
// передать результат прямо в ShowVerse/restore, не доставая стих повторно.
type VerseSearchHit struct {
	ShownVerse
	Score   float64        `json:"score"`
	Matches []search.Match `json:"matches"`
}

// CoupletSearchHit — найденный куплет вместе с песней, к которой он относится.
type CoupletSearchHit struct {
	ShownCouplet
	Score   float64        `json:"score"`
	Matches []search.Match `json:"matches"`
}

// openSearchIndex поднимает индекс и, если он пуст, запускает первичную сборку
// в фоне. Сборка идёт именно в фоне: она занимает секунды, а окно оператора
// должно открыться сразу — до готовности индекса работает вся программа, кроме
// самого поиска.
func (g *DbHandler) openSearchIndex() error {
	idx, err := search.Open(searchIndexPath)
	if err != nil {
		return err
	}
	g.searchIdx = idx

	count, err := idx.Count()
	if err != nil {
		return err
	}
	if count == 0 {
		go func() {
			if err := g.RebuildSearchIndex(); err != nil {
				log.Println("Error building search index", err.Error())
			}
		}()
	}
	return nil
}

// RebuildSearchIndex собирает индекс с нуля. Вызывается автоматически при
// пустом индексе и вручную из настроек, если индекс разошёлся с базой.
//
// Две пересборки одновременно недопустимы: Reset закрывает и пересоздаёт
// индекс, так что параллельная сборка писала бы в уже закрытый. Кнопка в
// настройках блокируется на время работы, но это защита интерфейса — здесь
// стоит настоящая.
func (g *DbHandler) RebuildSearchIndex() (err error) {
	if g.searchIdx == nil {
		return fmt.Errorf("поисковый индекс не открыт")
	}
	if !g.rebuildMu.TryLock() {
		return fmt.Errorf("пересборка индекса уже идёт")
	}
	defer g.rebuildMu.Unlock()

	// Об обрыве надо сказать интерфейсу. Прогресс он считает по событиям, и без
	// этого сообщения строка поиска осталась бы с вечным «индексация N из M», а
	// кнопка пересборки — заблокированной до перезапуска программы.
	defer func() {
		if err != nil {
			g.emit("search_index_failed", err.Error())
		}
	}()

	if err := g.searchIdx.Reset(); err != nil {
		return err
	}

	var translations []models.Translation
	if err := inits.DB.Find(&translations).Error; err != nil {
		return err
	}

	// Песни идут одним шагом сверх переводов — отсюда +1.
	total := len(translations) + 1
	g.emit("search_index_progress", map[string]any{"done": 0, "total": total})
	for n, t := range translations {
		if err := g.indexTranslation(t.ID); err != nil {
			return err
		}
		g.emit("search_index_progress", map[string]any{"done": n + 1, "total": total, "name": t.Name})
	}

	if err := g.indexAllCouplets(); err != nil {
		return err
	}
	g.emit("search_index_progress", map[string]any{"done": total, "total": total, "name": "Песни"})
	g.emit("search_index_ready", nil)
	return nil
}

// indexTranslation загружает все стихи перевода одним запросом и отправляет их
// в индекс пачками. Стихи читаются через join, а не через GORM-preload: нужен
// только текст и идентификатор, разворачивать дерево книга→глава→стих незачем.
func (g *DbHandler) indexTranslation(translationId uint) error {
	rows, err := inits.DB.
		Table("verses").
		Select("verses.id, verses.text").
		Joins("join chapters on chapters.id = verses.chapter_id").
		Joins("join books on books.id = chapters.book_id").
		Where("books.translation_id = ? AND verses.deleted_at IS NULL", translationId).
		Rows()
	if err != nil {
		return err
	}
	defer rows.Close()

	batch := docBatcher{idx: g.searchIdx}
	for rows.Next() {
		var id uint
		var text string
		if err := rows.Scan(&id, &text); err != nil {
			return err
		}
		err := batch.add(search.Doc{
			ID:            id,
			Kind:          search.KindVerse,
			TranslationID: translationId,
			Text:          text,
		})
		if err != nil {
			return err
		}
	}
	// rows.Err() обязателен и обязательно ДО хвостовой пачки: перебор может
	// оборваться на ошибке драйвера, и тогда всё прочитанное — половина
	// перевода. Раньше эта проверка стояла в ветке «хвост пуст», то есть
	// срабатывала только при числе стихов, кратном размеру пачки.
	if err := rows.Err(); err != nil {
		return err
	}
	return batch.flush()
}

// docBatcher копит документы и сбрасывает их в индекс пачками: Bleve заметно
// быстрее батчами, чем поштучно, а размер пачки один на всех.
type docBatcher struct {
	idx  *search.Index
	docs []search.Doc
}

func (b *docBatcher) add(d search.Doc) error {
	b.docs = append(b.docs, d)
	if len(b.docs) < indexBatchSize {
		return nil
	}
	return b.flush()
}

func (b *docBatcher) flush() error {
	if len(b.docs) == 0 {
		return nil
	}
	if err := b.idx.PutBatch(b.docs); err != nil {
		return err
	}
	b.docs = b.docs[:0]
	return nil
}

// indexAllCouplets переиндексирует все куплеты всех песен.
func (g *DbHandler) indexAllCouplets() error {
	var couplets []models.Couplet
	if err := inits.DB.Find(&couplets).Error; err != nil {
		return err
	}
	batch := docBatcher{idx: g.searchIdx}
	for _, c := range couplets {
		if err := batch.add(search.Doc{ID: c.ID, Kind: search.KindCouplet, Text: c.Text}); err != nil {
			return err
		}
	}
	return batch.flush()
}

// Хуки синхронизации. Все они «тихие»: рассинхрон индекса не должен ронять
// операцию, ради которой оператор нажал кнопку, поэтому ошибка логируется, а
// не возвращается. Худшее последствие — устаревшая строка в поиске до
// следующей пересборки.

func (g *DbHandler) indexCoupletSilent(c models.Couplet) {
	if g.searchIdx == nil {
		return
	}
	if err := g.searchIdx.Put(search.Doc{ID: c.ID, Kind: search.KindCouplet, Text: c.Text}); err != nil {
		log.Println("Error indexing couplet", err.Error())
	}
}

// unindexSilent убирает из индекса документы одного типа. Удаление пакетное:
// при сносе перевода идентификаторов больше тридцати тысяч, и поштучно это
// заняло бы индекс на минуты прямо во время работы оператора.
func (g *DbHandler) unindexSilent(kind string, ids ...uint) {
	if g.searchIdx == nil || len(ids) == 0 {
		return
	}
	if err := g.searchIdx.DeleteBatch(kind, ids); err != nil {
		log.Println("Error removing from index", kind, err.Error())
	}
}

// reindexSongSilent переиндексирует все куплеты песни. Нужен там, где меняется
// сразу набор блоков (ReplaceCouplets) или сдвигаются номера.
func (g *DbHandler) reindexSongSilent(songId uint) {
	if g.searchIdx == nil {
		return
	}
	var couplets []models.Couplet
	if err := inits.DB.Where("song_id = ?", songId).Find(&couplets).Error; err != nil {
		log.Println("Error loading couplets for reindex", err.Error())
		return
	}
	docs := make([]search.Doc, 0, len(couplets))
	for _, c := range couplets {
		docs = append(docs, search.Doc{ID: c.ID, Kind: search.KindCouplet, Text: c.Text})
	}
	if len(docs) == 0 {
		return
	}
	if err := g.searchIdx.PutBatch(docs); err != nil {
		log.Println("Error reindexing song", err.Error())
	}
}

// indexTranslationSilent индексирует перевод после импорта.
//
// Берёт тот же замок, что и полная пересборка, и именно блокирующий: первый
// запуск строит индекс в фоне несколько секунд, и импорт, попавший в это окно,
// либо писал бы в индекс, который Reset вот-вот снесёт, либо в уже закрытый.
// Импорту правильно подождать — пропустить индексацию значит оставить свежий
// перевод ненаходимым без единого сообщения об ошибке.
func (g *DbHandler) indexTranslationSilent(translationId uint) {
	if g.searchIdx == nil {
		return
	}
	g.rebuildMu.Lock()
	defer g.rebuildMu.Unlock()

	if err := g.indexTranslation(translationId); err != nil {
		log.Println("Error indexing translation", err.Error())
		return
	}
	g.emit("search_index_ready", nil)
}

// verseIdsOfTranslation собирает идентификаторы стихов перевода — вызывается
// перед каскадным удалением, пока строки ещё на месте.
func verseIdsOfTranslation(translationId uint) []uint {
	var ids []uint
	err := inits.DB.
		Table("verses").
		Joins("join chapters on chapters.id = verses.chapter_id").
		Joins("join books on books.id = chapters.book_id").
		Where("books.translation_id = ?", translationId).
		Pluck("verses.id", &ids).Error
	if err != nil {
		log.Println("Error collecting verse ids", err.Error())
		return nil
	}
	return ids
}

// SearchVerses ищет стихи в пределах перевода. Пустой запрос возвращает пустой
// список, а не всю Библию.
func (g *DbHandler) SearchVerses(query string, translationId float32, limit float32) []VerseSearchHit {
	if g.searchIdx == nil || query == "" {
		return []VerseSearchHit{}
	}

	hits, terms, err := g.searchIdx.Search(query, search.Opts{
		Kind:          search.KindVerse,
		TranslationID: uint(translationId),
		Limit:         int(limit),
		Fuzzy:         true,
	})
	if err != nil {
		log.Println("Error searching verses", err.Error())
		return []VerseSearchHit{}
	}
	if len(hits) == 0 {
		return []VerseSearchHit{}
	}

	// Индекс хранит только идентификаторы — тексты и связи читаются из базы
	// одним запросом, порядок восстанавливается по выдаче поиска.
	ids := make([]uint, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.ID)
	}

	var verses []models.Verse
	if err := inits.DB.Where("id IN ?", ids).Find(&verses).Error; err != nil {
		log.Println("Error loading found verses", err.Error())
		return []VerseSearchHit{}
	}
	versesById := make(map[uint]models.Verse, len(verses))
	chapterIds := make([]uint, 0, len(verses))
	for _, v := range verses {
		versesById[v.ID] = v
		chapterIds = append(chapterIds, v.ChapterId)
	}

	// Ошибки здесь проверяются наравне со стихами: без книги и главы попадание
	// выглядит нормальным, но переход по нему уводит в пустую главу — оператору
	// лучше пустая выдача, чем результат, который никуда не ведёт.
	var chapters []models.Chapter
	if err := inits.DB.Where("id IN ?", chapterIds).Find(&chapters).Error; err != nil {
		log.Println("Error loading chapters for found verses", err.Error())
		return []VerseSearchHit{}
	}
	chaptersById := make(map[uint]models.Chapter, len(chapters))
	bookIds := make([]uint, 0, len(chapters))
	for _, c := range chapters {
		chaptersById[c.ID] = c
		bookIds = append(bookIds, c.BookId)
	}

	var books []models.Book
	if err := inits.DB.Where("id IN ?", bookIds).Find(&books).Error; err != nil {
		log.Println("Error loading books for found verses", err.Error())
		return []VerseSearchHit{}
	}
	booksById := make(map[uint]models.Book, len(books))
	for _, b := range books {
		booksById[b.ID] = b
	}

	translation := models.Translation{}
	if err := inits.DB.First(&translation, uint(translationId)).Error; err != nil {
		log.Println("Error loading translation for found verses", err.Error())
		return []VerseSearchHit{}
	}

	out := make([]VerseSearchHit, 0, len(hits))
	for _, h := range hits {
		verse, ok := versesById[h.ID]
		if !ok {
			continue // строка исчезла из базы, а индекс ещё не догнал
		}
		chapter := chaptersById[verse.ChapterId]
		out = append(out, VerseSearchHit{
			ShownVerse: ShownVerse{
				Verse:       verse,
				Chapter:     chapter,
				Book:        booksById[chapter.BookId],
				Translation: translation,
			},
			Score:   h.Score,
			Matches: search.Highlight(verse.Text, terms),
		})
	}
	return out
}

// SearchCouplets ищет по тексту куплетов всех песен.
func (g *DbHandler) SearchCouplets(query string, limit float32) []CoupletSearchHit {
	if g.searchIdx == nil || query == "" {
		return []CoupletSearchHit{}
	}

	hits, terms, err := g.searchIdx.Search(query, search.Opts{
		Kind:  search.KindCouplet,
		Limit: int(limit),
		Fuzzy: true,
	})
	if err != nil {
		log.Println("Error searching couplets", err.Error())
		return []CoupletSearchHit{}
	}
	if len(hits) == 0 {
		return []CoupletSearchHit{}
	}

	ids := make([]uint, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.ID)
	}

	var couplets []models.Couplet
	if err := inits.DB.Where("id IN ?", ids).Find(&couplets).Error; err != nil {
		log.Println("Error loading found couplets", err.Error())
		return []CoupletSearchHit{}
	}
	coupletsById := make(map[uint]models.Couplet, len(couplets))
	songIds := make([]uint, 0, len(couplets))
	for _, c := range couplets {
		coupletsById[c.ID] = c
		songIds = append(songIds, c.SongId)
	}

	// Как и у стихов: без песни переход по найденному куплету ведёт в никуда.
	var songs []models.Song
	if err := inits.DB.Where("id IN ?", songIds).Find(&songs).Error; err != nil {
		log.Println("Error loading songs for found couplets", err.Error())
		return []CoupletSearchHit{}
	}
	songsById := make(map[uint]models.Song, len(songs))
	for _, s := range songs {
		songsById[s.ID] = s
	}

	out := make([]CoupletSearchHit, 0, len(hits))
	for _, h := range hits {
		couplet, ok := coupletsById[h.ID]
		if !ok {
			continue
		}
		out = append(out, CoupletSearchHit{
			ShownCouplet: ShownCouplet{
				Couplet: couplet,
				Song:    songsById[couplet.SongId],
			},
			Score:   h.Score,
			Matches: search.Highlight(couplet.Text, terms),
		})
	}
	return out
}
