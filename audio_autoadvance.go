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
// Владение: ровно один из {tryAdvance, invalidatePending, invalidatePendingForTrack}
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
// invalidatePending (при отмене). Под pendingMu ровно один из конкурентных
// вызовов увидит ненулевой указатель — mutex сериализует их так же, как
// player.nextGen сериализует конкурирующие play/stop/seek: кто взял мьютекс
// раньше, тот и владеет содержимым единолично, второй увидит уже nil и
// ничего не тронет. Отдельного протокола отмены (atomic CAS и т.п.) не
// нужно — именно поэтому в pendingNext нет мьютекса, только pendingMu
// снаружи.
func (a *AudioService) takePending() *pendingNext {
	a.pendingMu.Lock()
	pn := a.pending
	a.pending = nil
	a.pendingMu.Unlock()
	return pn
}

// invalidatePending отменяет и закрывает текущий pending, если он есть.
// Вызывается из Play/Stop/SetDevice/мутаций плейлиста — везде, где решение
// оператора могло сделать заранее подготовленный "следующий" трек неверным.
// Блокируется на <-pn.ready (обычно уже закрыт: подготовка почти всегда
// успевает раньше явного действия оператора) — вызывающие все на стороне
// обычных Wails-вызовов, не на аудиопотоке и не под p.mu, так что блокировка
// здесь безопасна (И2 говорит про malgo/файлы/БД под p.mu, а не про это).
func (a *AudioService) invalidatePending() {
	pn := a.takePending()
	if pn == nil {
		return
	}
	pn.canceled.Store(true)
	<-pn.ready
	pn.closeSrc()
}

// invalidatePendingForTrack — как invalidatePending, но только если pending
// сейчас держит открытым именно этот трек. Нужна RemoveTrack: без неё
// os.Remove мог бы упасть с sharing violation на Windows, если удаляемый
// трек — не текущий играющий (это RemoveTrack уже проверяет через
// isCurrentTrack), а именно заранее подготовленный "следующий".
func (a *AudioService) invalidatePendingForTrack(trackId uint) {
	a.pendingMu.Lock()
	pn := a.pending
	if pn == nil || pn.trackId != trackId {
		a.pendingMu.Unlock()
		return
	}
	a.pending = nil
	a.pendingMu.Unlock()

	pn.canceled.Store(true)
	<-pn.ready
	pn.closeSrc()
}

// preparePendingNext подбирает элемент плейлиста после afterItemId (с учётом
// AutoAdvance/Loop) и, если он есть, заранее открывает его файл в фоне.
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
// <-pn.ready вместо ложного "нечего продолжать".
//
// ⚠️ Осознанное упрощение: NextTitle и подготовка pending считаются ТОЛЬКО
// когда у плейлиста AutoAdvance включён. Плейлист без AutoAdvance не покажет
// «далее: …» даже если в нём есть следующий трек — план описывает
// pending/NextTitle именно в контексте автоперехода, а не как отдельную
// функцию предпросмотра очереди.
func (a *AudioService) preparePendingNext(playlistId, afterItemId uint) {
	var playlist models.Playlist
	if err := inits.DB.First(&playlist, playlistId).Error; err != nil || !playlist.AutoAdvance {
		a.pl.setNextTitle("")
		return
	}

	next, ok := a.nextPlaylistItem(playlistId, afterItemId, playlist.Loop)
	if !ok {
		a.pl.setNextTitle("")
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

	// Более старый pending (если preparePendingNext почему-то вызвали дважды
	// для одного и того же играющего трека) — сторонний объект, который уже
	// никому не нужен: отменяем и закрываем его независимо от нашей
	// собственной подготовки. takePending делает снятие старого и
	// публикацию pn нераздельными для внешнего наблюдателя (см. выше).
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
// делает полный проход по файлу. Выполняется в отдельной горутине (запущена
// из preparePendingNext уже ПОСЛЕ синхронной публикации pn в a.pending — см.
// комментарий там), поэтому здесь нет ни одного обращения к a.pending. item —
// уже с преloaded Track (nextPlaylistItem), так что здесь ни одного
// обращения к БД, только файловый ввод-вывод.
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

// tryAdvance подхватывает уже подготовленный pending ровно тогда, когда
// finished — тот самый элемент, для которого его готовили. gen — то самое
// поколение, что finishIfCurrent зарезервировал в одной с собой критической
// секции (см. комментарий там): startTrack ниже либо победит (если никто не
// вмешался), либо честно проиграет более новому Stop()/Play() — обычный И4.
//
// Возвращает false, если автоперехода не случилось (нет пары/автоперехода
// нет/ошибка) — тогда watchPlayerEvents эмитит audio_stopped сам. true — во
// всех остальных случаях, включая "нас обогнали": тогда состояние уже
// расставлено тем, кто обогнал, и эмитить "стоп" самим не нужно.
func (a *AudioService) tryAdvance(finished trackMeta, gen uint64) bool {
	a.pendingMu.Lock()
	pn := a.pending
	if pn == nil || pn.forItemId != finished.itemId {
		a.pendingMu.Unlock()
		return false
	}
	a.pending = nil
	a.pendingMu.Unlock()

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
