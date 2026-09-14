package main

import (
	"fmt"
	"sync/atomic"

	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/paths"

	"github.com/gopxl/beep/v2"
)

// pendingNext — трек, следующий за играющим в плейлисте с AutoAdvance,
// заранее открытый и декодированный в фоне. decodeExt дорог — полный проход
// по файлу (И3) — и делать это только по приходу "трек доиграл" означало бы
// дыру в сотни миллисекунд между треками независимо от фейдов (план, этап 4:
// «следующий трек начинает открываться заранее»).
//
// ⚠️ Модель состояния (рефакторинг): pending — НЕ кэш поверх честного пути,
// а единственный механизм автоперехода (см. tryAdvance). Любая мутация,
// способная изменить ответ на вопрос «что идёт после currently-playing
// элемента», ОБЯЗАНА пройти через refreshPending, а не просто выбросить
// pending — иначе следующий done придёт к пустому pending, tryAdvance не
// найдёт пару и посчитает переход несостоявшимся: доигравший трек оборвётся
// тишиной, хотя дальше по списку есть на что переходить (см. buildPendingSync
// — синхронная страховка на этот самый случай, ценой заминки, а не тишины).
//
// Владение: ровно один из {tryAdvance, dropPending, dropPendingForTrack}
// когда-либо получает право закрыть pn.src — см. комментарий у AudioService.pending.
type pendingNext struct {
	forItemId  uint // done должен прийти именно от этого играющего элемента
	itemId     uint
	trackId    uint
	playlistId uint

	ready      chan struct{} // закрывается, когда подготовка завершена (успешно или с ошибкой)
	src        beep.StreamSeekCloser
	fileRate   beep.SampleRate
	gainDb     float64
	durationMs int
	err        error

	// Этап 5 (trim/фейды): fadeMs — FadeMs плейлиста на момент подготовки,
	// известен уже в preparePendingNext (playlist уже загружен для проверки
	// AutoAdvance). trimStartFileFrame/totalSamples/fadeSamples считает
	// decodePendingNext ПОСЛЕ decodeExt — тем же способом, что и Play()
	// в audio_service.go, и по той же причине (нужны devRate/format.SampleRate,
	// которых нет до открытия файла).
	fadeMs             int
	trimStartFileFrame int
	totalSamples       int
	fadeSamples        int

	// canceled — Play()/Stop()/мутация плейлиста опередили автопереход.
	// Проверяется самой decodePendingNext ПОСЛЕ decodeExt: decode нельзя
	// прервать на середине, но можно не устанавливать то, что уже никому не
	// нужно, и закрыть src немедленно, а не оставлять висеть открытый файл.
	canceled atomic.Bool
}

func (pn *pendingNext) closeSrc() {
	if pn.src != nil {
		pn.src.Close()
	}
}

// takePending — единственная точка, где a.pending читается И одновременно
// снимается: используется и tryAdvance (при успешном подхвате), и
// dropPending/preparePendingNext (при отмене/замене). Под pendingMu ровно
// один из конкурентных вызовов увидит ненулевой указатель — mutex
// сериализует их так же, как player.nextGen сериализует конкурирующие
// play/stop/seek: кто взял мьютекс раньше, тот и владеет содержимым
// единолично, второй увидит уже nil и ничего не тронет. Отдельного протокола
// отмены (atomic CAS и т.п.) не нужно — именно поэтому в pendingNext нет
// мьютекса, только pendingMu снаружи.
func (a *AudioService) takePending() *pendingNext {
	a.pendingMu.Lock()
	pn := a.pending
	a.pending = nil
	a.pendingMu.Unlock()
	return pn
}

// dropPending отменяет и закрывает текущий pending БЕЗ пересчёта — законно
// только там, где воспроизведения больше нет и считать новый "следующий"
// не для чего: Stop, FadeOutStop, SetDevice, ServiceShutdown, провал Seek.
// Мутации плейлиста (Reorder/Remove/Add/SetFlags и т.п.), где что-то
// продолжает играть, обязаны звать refreshPending, а не эту функцию — см.
// предупреждение у pendingNext выше.
//
// nextTitle сбрасывается здесь же, а не отдельным вызовом на месте: пара
// (pending, nextTitle) должна меняться одной операцией — иначе PlayerState
// между вызовами показал бы оператору «далее: …» для трека, которого уже
// нет.
//
// Блокируется на <-pn.ready (обычно уже закрыт: подготовка почти всегда
// успевает раньше явного действия оператора) — вызывающие все на стороне
// обычных Wails-вызовов, не на аудиопотоке и не под p.mu, так что блокировка
// здесь безопасна (И2 говорит про malgo/файлы/БД под p.mu, а не про это).
func (a *AudioService) dropPending() {
	pn := a.takePending()
	if a.pl != nil {
		a.pl.setNextTitle("")
	}
	if pn == nil {
		return
	}
	pn.canceled.Store(true)
	<-pn.ready
	pn.closeSrc()
}

// dropPendingForTrack — как dropPending, но только если pending сейчас
// держит открытым именно этот трек. Нужна UpdateTrack/RemoveTrack/
// deleteTrackIfUnused: без неё os.Remove мог бы упасть с sharing violation
// на Windows, если трогаемый трек — не текущий играющий (это уже проверяет
// isCurrentTrack), а именно заранее подготовленный "следующий".
//
// Возвращает playlistId/afterItemId дропнутого pending и dropped=true, если
// был что дропать — вызывающий использует их, чтобы пересчитать pending
// ПОСЛЕ того, как сама причина дропа (правка файла, удаление строки)
// применится к БД, иначе refreshPending пересчитал бы ответ по ещё не
// изменённым данным.
func (a *AudioService) dropPendingForTrack(trackId uint) (playlistId, afterItemId uint, dropped bool) {
	a.pendingMu.Lock()
	pn := a.pending
	if pn == nil || pn.trackId != trackId {
		a.pendingMu.Unlock()
		return 0, 0, false
	}
	a.pending = nil
	a.pendingMu.Unlock()

	playlistId, afterItemId = pn.playlistId, pn.forItemId
	pn.canceled.Store(true)
	<-pn.ready
	pn.closeSrc()
	if a.pl != nil {
		a.pl.setNextTitle("")
	}
	return playlistId, afterItemId, true
}

// refreshPending — единственная точка, которую обязаны звать мутации
// плейлиста (Reorder/Remove/Add/SetPlaylistFlags/правка trim трека), когда
// что-то из playlistId продолжает играть: пересчитывает ответ на "что дальше
// после afterItemId" и, если он не изменился, оставляет уже открытый
// декодер как есть (см. preparePendingNext — там же и дедупликация). В
// отличие от dropPending, никогда не оставляет автопереход без пары молча —
// именно это было причиной тишины после безобидной перестановки/удаления
// (см. предупреждение у pendingNext).
func (a *AudioService) refreshPending(playlistId, afterItemId uint) {
	a.preparePendingNext(playlistId, afterItemId)
}

// refreshPendingForPlaylist — общий охранник для мутаций плейлиста:
// пересчитывает pending, только если сейчас играет (или на паузе) элемент
// ИМЕННО этого плейлиста. Мутация чужого плейлиста не должна задевать
// pending того, что реально звучит сейчас.
func (a *AudioService) refreshPendingForPlaylist(playlistId uint) {
	if a.pl == nil {
		return
	}
	if pid, itemId, ok := a.pl.currentItem(); ok && pid == playlistId {
		a.refreshPending(playlistId, itemId)
	}
}

// preparePendingNext подбирает элемент плейлиста после afterItemId (с учётом
// AutoAdvance/Loop) и, если он есть, заранее открывает его файл в фоне.
// Вызывается и при старте нового трека (Play/tryAdvance), и как реализация
// refreshPending при мутациях плейлиста, продолжающих играть.
//
// ⚠️ Дедупликация: если уже открытый a.pending — это ровно то, что мы бы
// подготовили сейчас (тот же forItemId ждёт того же itemId), декодер НЕ
// переоткрывается. Без неё безобидная мутация где-то в плейлисте
// (переименование трека, добавление другого элемента в конец, повторный
// пересчёт после SetPlaylistFlags без реального изменения ответа) рвала бы
// уже готовый файл заново — не критично для звука (idle-время между вызовом
// и concatMs невелико), но лишний decodeExt (И3, полный проход по файлу) на
// каждую мелкую правку плейлиста недопустим при живом эфире.
//
// ⚠️ Регистрация a.pending (ниже) — СИНХРОННАЯ, до запуска горутины с
// decodeExt. Раньше сам pendingNext создавался и публиковался В горутине, и
// между "решили, что дальше вот этот трек" и "фактически положили указатель
// в a.pending" оставалось окно в несколько микросекунд планировщика Go. На
// настоящих файлах (секунды звучания) это окно никогда не имело значения, но
// теоретически — и практически, в тесте с треками в сотни сэмплов — трек мог
// доиграть быстрее, чем горутина успевала запуститься: tryAdvance видел
// a.pending == nil и решал, что продолжать нечем, хотя решение уже было
// принято и pn секундой позже всё равно появился бы. Синхронная регистрация
// закрывает окно целиком: к моменту, когда preparePendingNext возвращает
// управление (Play/tryAdvance), a.pending уже указывает на pn (ещё не
// готовый — ready не закрыт), и конкурентный tryAdvance просто дождётся
// <-pn.ready вместо ложного "нечего продолжать" (а с этапа "СRITICAL" ниже —
// ещё и вместо тишины: см. buildPendingSync).
//
// ⚠️ Осознанное упрощение: NextTitle и подготовка pending считаются ТОЛЬКО
// когда у плейлиста AutoAdvance включён. Плейлист без AutoAdvance не покажет
// «далее: …» даже если в нём есть следующий элемент — план описывает
// pending/NextTitle именно в контексте автоперехода, а не как отдельную
// функцию предпросмотра очереди.
func (a *AudioService) preparePendingNext(playlistId, afterItemId uint) {
	var playlist models.Playlist
	if err := inits.DB.First(&playlist, playlistId).Error; err != nil || !playlist.AutoAdvance {
		a.dropPending()
		return
	}

	next, ok := a.nextPlaylistItem(playlistId, afterItemId, playlist.Loop)
	if !ok {
		a.dropPending()
		return
	}

	a.pendingMu.Lock()
	current := a.pending
	a.pendingMu.Unlock()
	if current != nil && current.forItemId == afterItemId && current.itemId == next.ID {
		// Ответ не изменился — уже открытый декодер остаётся как есть,
		// только заголовок пересчитан (дёшево, идемпотентно).
		a.pl.setNextTitle(next.Track.Title)
		return
	}

	a.pl.setNextTitle(next.Track.Title)

	// Фейд считаем для ЭТОГО подготавливаемого трека (next) — на момент,
	// когда играть будет уже он.
	fadeMs := a.fadeMsBefore(playlist, next.ID)

	pn := &pendingNext{
		forItemId:  afterItemId,
		itemId:     next.ID,
		trackId:    next.TrackId,
		playlistId: playlistId,
		ready:      make(chan struct{}),
		gainDb:     next.Track.GainDb,
		durationMs: trimmedDurationMs(next.Track), // этап 5: UI считает от TrimStartMs
		fadeMs:     fadeMs,
	}

	// Более старый pending (если preparePendingNext вызвали дважды для
	// одного и того же afterItemId с другим ответом, либо для другого
	// играющего трека) — сторонний объект, который уже никому не нужен:
	// отменяем и закрываем его независимо от нашей собственной подготовки.
	// takePending делает снятие старого и публикацию pn нераздельными для
	// внешнего наблюдателя (см. выше).
	old := a.takePending()
	a.pendingMu.Lock()
	a.pending = pn
	a.pendingMu.Unlock()
	if old != nil {
		old.canceled.Store(true)
		go func() { <-old.ready; old.closeSrc() }()
	}

	go a.decodePendingNext(pn, next)
}

// fadeMsBefore — FadeMs плейлиста для трека, который сейчас доигрывает
// элемент afterItemId. Единственное место, где живёт правило «фейд — только
// когда есть куда переходить»; его спрашивают оба пути старта трека (Play в
// audio_service.go и preparePendingNext выше), и разъезжаться этим двум
// ответам нельзя: цепочка строится по одному, а автопереход — по другому.
//
// ⚠️ Ноль, если автопереход выключен или следующего элемента нет: план
// описывает фейд как "перед автопереходом", а не как безусловное затухание в
// конце каждого трека — иначе последний трек плейлиста (или единственный
// трек без AutoAdvance) тоже уходил бы в фейд, хотя после него ничего не
// звучит.
func (a *AudioService) fadeMsBefore(playlist models.Playlist, afterItemId uint) int {
	if !playlist.AutoAdvance {
		return 0
	}
	if _, hasNext := a.nextPlaylistItem(playlist.ID, afterItemId, playlist.Loop); !hasNext {
		return 0
	}
	return playlist.FadeMs
}

// nextPlaylistItem вычисляет элемент, следующий за afterItemId, в порядке
// Position ASC. Loop оборачивает после последнего к первому; без Loop конец
// списка означает "следующего нет". afterItemId, не найденный в списке
// (удалён/переставлен в другом месте между вызовами), тоже даёт "нет" — не
// пытаемся угадывать позицию по устаревшим данным.
func (a *AudioService) nextPlaylistItem(playlistId, afterItemId uint, loop bool) (models.PlaylistItem, bool) {
	var items []models.PlaylistItem
	if err := inits.DB.Preload("Track").Where("playlist_id = ?", playlistId).Order("position ASC").Find(&items).Error; err != nil || len(items) == 0 {
		return models.PlaylistItem{}, false
	}
	idx := itemIndex(items, afterItemId)
	if idx < 0 {
		return models.PlaylistItem{}, false
	}
	if idx+1 < len(items) {
		return items[idx+1], true
	}
	if loop {
		return items[0], true
	}
	return models.PlaylistItem{}, false
}

// decodePendingNext — единственная дорогая (И3) часть подготовки: decodeExt
// делает полный проход по файлу. Обычно запускается в отдельной горутине из
// preparePendingNext (уже ПОСЛЕ синхронной публикации pn в a.pending — см.
// комментарий там); buildPendingSync (ниже) зовёт её напрямую, синхронно, как
// запасной путь tryAdvance. item — уже с преloaded Track (nextPlaylistItem),
// так что здесь ни одного обращения к БД, только файловый ввод-вывод.
func (a *AudioService) decodePendingNext(pn *pendingNext, item models.PlaylistItem) {
	pn.err = a.fillPendingNext(pn, item.Track)

	if pn.canceled.Load() {
		pn.closeSrc()
		pn.src = nil
	}
	close(pn.ready)
}

// fillPendingNext — сама подготовка, отдельно от бухгалтерии готовности
// (pn.err/canceled/close(ready)) выше: она обязана выполниться при любом
// исходе, а здесь на каждой неудаче достаточно раннего возврата. На ошибке
// pn.src остаётся nil, и pn.err расскажет tryAdvance, почему подхватывать
// нечего.
func (a *AudioService) fillPendingNext(pn *pendingNext, track models.AudioTrack) error {
	mediaDir, err := paths.MediaDir()
	if err != nil {
		return err
	}
	src, format, trimStartFrame, err := openTrackSource(mediaDir, track)
	if err != nil {
		return err
	}

	pn.src = src
	pn.fileRate = format.SampleRate
	pn.trimStartFileFrame = trimStartFrame
	pn.totalSamples, pn.fadeSamples = trackChainBounds(track, pn.fadeMs, a.pl.deviceRateSnapshot())
	return nil
}

// buildPendingSync — СИНХРОННЫЙ запасной путь tryAdvance на случай, когда
// честный (заранее подготовленный) pending для finished-элемента не найден —
// пропущенный refreshPending где-то в коде, AutoAdvance включили только что,
// либо просто ещё не успел подготовиться. Повторяет вычисление
// preparePendingNext, но без горутины и без публикации в a.pending: дороже
// (decodeExt — полный проход по файлу, И3) — оператор получит заминку в
// сотни миллисекунд вместо мгновенного перехода, — зато НИКОГДА не тишину
// там, где формально есть на что переходить. Это единственная причина, по
// которой пропущенная где-то инвалидация pending — баг производительности, а
// не баг функциональности (см. предупреждение у pendingNext).
func (a *AudioService) buildPendingSync(playlistId, afterItemId uint) *pendingNext {
	var playlist models.Playlist
	if err := inits.DB.First(&playlist, playlistId).Error; err != nil || !playlist.AutoAdvance {
		return nil
	}
	next, ok := a.nextPlaylistItem(playlistId, afterItemId, playlist.Loop)
	if !ok {
		return nil
	}

	fadeMs := a.fadeMsBefore(playlist, next.ID)
	pn := &pendingNext{
		forItemId:  afterItemId,
		itemId:     next.ID,
		trackId:    next.TrackId,
		playlistId: playlistId,
		ready:      make(chan struct{}),
		gainDb:     next.Track.GainDb,
		durationMs: trimmedDurationMs(next.Track),
		fadeMs:     fadeMs,
	}
	a.decodePendingNext(pn, next) // синхронно — decodePendingNext сам закрывает ready
	return pn
}

// tryAdvance подхватывает подготовленный pending ровно тогда, когда finished
// — тот самый элемент, для которого его готовили; если такого pending нет
// (см. buildPendingSync выше), считает и открывает следующий трек синхронно
// прямо здесь, а не сдаётся молча. gen — то самое поколение, что
// finishIfCurrent зарезервировал в одной с собой критической секции (см.
// комментарий там): startTrack ниже либо победит (если никто не вмешался),
// либо честно проиграет более новому Stop()/Play() — обычный И4.
//
// Возвращает false, если автоперехода не случилось (нет пары/автоперехода
// нет/ошибка) — тогда watchPlayerEvents эмитит audio_stopped сам. true — во
// всех остальных случаях, включая "нас обогнали": тогда состояние уже
// расставлено тем, кто обогнал, и эмитить "стоп" самим не нужно.
func (a *AudioService) tryAdvance(finished trackMeta, gen uint64) bool {
	a.pendingMu.Lock()
	pn := a.pending
	if pn != nil && pn.forItemId == finished.itemId {
		a.pending = nil
		a.pendingMu.Unlock()
	} else {
		a.pendingMu.Unlock()
		// CRITICAL (см. предупреждение у pendingNext): честный pending не
		// подготовлен — страховка вместо тишины, ценой decodeExt синхронно.
		pn = a.buildPendingSync(finished.playlistId, finished.itemId)
		if pn == nil {
			return false
		}
	}

	<-pn.ready
	if pn.err != nil {
		a.emit("audio_error", fmt.Sprintf("не удалось открыть следующий трек: %s", pn.err.Error()))
		pn.closeSrc()
		return false
	}
	if pn.canceled.Load() {
		pn.closeSrc()
		return false
	}

	devRate := a.pl.deviceRateSnapshot()
	chain := buildChain(pn.src, pn.fileRate, devRate, pn.gainDb, pn.totalSamples, pn.fadeSamples, a.pl.done, gen)
	meta := trackMeta{
		trackId:            pn.trackId,
		playlistId:         pn.playlistId,
		itemId:             pn.itemId,
		durationMs:         pn.durationMs,
		fileRate:           pn.fileRate,
		gainDb:             pn.gainDb,
		trimStartFileFrame: pn.trimStartFileFrame,
		totalSamples:       pn.totalSamples,
		fadeSamples:        pn.fadeSamples,
	}
	installed, staleSrc := a.pl.startTrack(gen, chain, pn.src, meta)
	if staleSrc != nil {
		staleSrc.Close()
	}
	if !installed {
		return true // обогнали Stop()/новый Play() — состояние уже расставлено ими
	}

	a.emit("audio_track_changed", a.State())
	a.preparePendingNext(pn.playlistId, pn.itemId)
	return true
}
