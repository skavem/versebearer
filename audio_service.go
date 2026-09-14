package main

import (
	"fmt"
	"log"
	"math"
	"path/filepath"
	"sync"
	"time"

	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/paths"

	"github.com/gopxl/beep/v2"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// AudioService — Wails-сервис вкладки «Звук». pl — движок, появившийся в
// этапе 2 (см. audio_player.go/audio_device.go); он создаётся сразу в
// NewAudioService, а не лениво при первом Play — иначе SetVolume, вызванный
// до первого воспроизведения, был бы некуда применить.
type AudioService struct {
	app *application.App
	pl  *player

	// volSaveTimer debounces GlobalState.AudioVolume persistence — по
	// образцу winSaveTimers в dbHandler.go: слайдер громкости не должен
	// бить SQLite на каждое движение мыши.
	volSaveMu    sync.Mutex
	volSaveTimer *time.Timer

	// pendingMu/pending — этап 4 (автопереход), см. audio_autoadvance.go.
	pendingMu sync.Mutex
	pending   *pendingNext

	// quit останавливает watchPlayerEvents при ServiceShutdown. Закрывается
	// ровно один раз; done/lost самих закрывать нельзя — data-колбэк malgo
	// может успеть отправить в них до Uninit устройства.
	quit chan struct{}
}

func NewAudioService() *AudioService {
	// Громкость и выбранное устройство читаем из GlobalState сразу:
	// inits.DB готова к этому моменту (открывается в package init(), до
	// main()). Ошибка чтения — не повод падать: откатываемся на полную
	// громкость (как и сама колонка по умолчанию,
	// models.GlobalState.AudioVolume `gorm:"default:1"`) и системное
	// устройство по умолчанию (пустая строка).
	vol := 1.0
	deviceId := ""
	var gs models.GlobalState
	if err := inits.DB.First(&gs, 1).Error; err == nil {
		vol = gs.AudioVolume
		deviceId = gs.AudioDeviceId
	}
	a := &AudioService{pl: newPlayer(vol, deviceId), quit: make(chan struct{})}
	go a.watchPlayerEvents()
	return a
}

// emit повторяет обёртку DbHandler.emit: до присваивания app (и в тестах)
// события молча глотаются, а не роняют вызов. app.Event.Emit сам по себе
// не блокирует вызывающего: EventProcessor.Emit (events.go) рассылает Go-
// слушателям из отдельной горутины и кладёт событие в mailbox — очередь,
// которая явно документирована как "processes messages in send order
// without blocking senders on the consumer" (internal/mailbox/mailbox.go).
// Поэтому вызывать emit прямо из watchPlayerEvents безопасно: она не
// застрянет на доставке во вьюху.
func (a *AudioService) emit(name string, data any) {
	if a.app != nil {
		a.app.Event.Emit(name, data)
	}
}

// watchPlayerEvents — единственный получатель асинхронных уведомлений
// движка: "трек доиграл" (done, с проверкой поколения — И4) и "устройство
// пропало" (lost). Оба канала обслуживаются одной горутиной, а не data-
// колбэком malgo (И2: там нельзя ни p.mu, ни emit синхронно). quit
// останавливает горутину при ServiceShutdown, чтобы она не дёргала emit
// после разборки приложения.
func (a *AudioService) watchPlayerEvents() {
	for {
		select {
		case gen := <-a.pl.done:
			oldSrc, finished, nextGen, changed := a.pl.finishIfCurrent(gen)
			if !changed {
				continue // устаревшее поколение — И4, кто-то уже успел вмешаться
			}
			// tryAdvance (этап 4) — либо подхватывает заранее подготовленный
			// следующий элемент плейлиста (AutoAdvance), либо честно
			// сообщает, что подхватывать нечего (false), и тогда это самый
			// обычный конец воспроизведения.
			advanced := a.tryAdvance(finished, nextGen)
			if oldSrc != nil {
				oldSrc.Close() // файловый IO — вне p.mu (И2/И3), мы уже не под ним
			}
			if !advanced {
				a.emit("audio_stopped", nil)
			}
		case <-a.pl.lost:
			// Устройство пропало само (выдернули провод, забрало другое
			// приложение) — не наш Stop()/SetDevice() (expectStop=false,
			// см. onDeviceStopped). closeDevice гарантирует, что следующий
			// Play() полноценно переоткроет устройство через ensureDevice, а
			// не решит, что deviceOpen==true и всё ещё работает. Тихой
			// подмены на другое устройство не делаем (план, этап 3).
			a.dropPending() // устройство пропало — воспроизведение прервано физически, считать следующий не для чего
			oldSrc := a.pl.forceIdleOnDeviceLost()
			if oldSrc != nil {
				oldSrc.Close()
			}
			a.pl.deviceLost.Store(true)
			a.pl.closeDevice()
			a.emit("audio_device_lost", nil)
		case <-a.quit:
			return
		}
	}
}

// ServiceShutdown, а не Close: Wails исключает из биндингов только
// ServiceName/ServiceStartup/ServiceShutdown/ServeHTTP — экспортированный
// Close() уехал бы во фронт как кнопка «разломать звук». Порядок важен:
// сначала честно остановить и закрыть src играющего трека (иначе он
// утекает открытым файловым хендлом), затем отменить отложенное сохранение
// громкости (иначе оно достучится до GORM/SQLite в уже разбираемом
// приложении), затем закрыть устройство синхронно (чтобы WASAPI не
// осталось захваченным после выхода — иначе следующим не выполнится
// отложенное закрытие поискового индекса, см. main.go), и только в конце
// остановить горутину-получатель событий.
func (a *AudioService) ServiceShutdown() error {
	a.dropPending() // иначе декодер заранее открытого следующего трека утекает открытым файловым хендлом
	if oldSrc := a.pl.stop(); oldSrc != nil {
		oldSrc.Close()
	}

	a.volSaveMu.Lock()
	if a.volSaveTimer != nil {
		a.volSaveTimer.Stop()
	}
	a.volSaveMu.Unlock()

	a.pl.closeDevice()
	close(a.quit)
	return nil
}

// PlayerState — то, что фронт вкладки «Звук» опрашивает раз в 250 мс.
// NextTitle и DeviceName вычисляются один раз при смене трека/устройства и
// хранятся полями player, а не запрашиваются здесь: State() не ходит в БД
// (иначе — GORM-запрос четыре раза в секунду всё время воспроизведения).
type PlayerState struct {
	Status     string  `json:"status"`
	TrackId    uint    `json:"trackId"`
	PlaylistId uint    `json:"playlistId"`
	ItemId     uint    `json:"itemId"`
	PositionMs int     `json:"positionMs"`
	DurationMs int     `json:"durationMs"`
	Volume     float64 `json:"volume"`
	Peak       float64 `json:"peak"`
	NextTitle  string  `json:"nextTitle"`
	DeviceName string  `json:"deviceName"`
	DeviceLost bool    `json:"deviceLost"`
}

// State возвращает текущий снимок плеера. Не ходит в БД (см. PlayerState) и
// не блокируется на устройстве — только на p.mu, который data-колбэк держит
// не дольше одного буфера.
//
// ⚠️ Побочный эффект: Peak читается через atomic.Swap(0) — это ЧТЕНИЕ-И-
// СБРОС, а не просто чтение (см. поле peak в player). Два вызова State()
// подряд никогда не покажут один и тот же пик дважды. В частности,
// Play()/emit("audio_track_changed", a.State()) ниже "съедает" окно пика за
// момент старта трека — безобидно (пик там всё равно ещё не накопился),
// но следующий читатель не должен удивляться и звать State() там, где
// важен именно текущий, не сбрасываемый пик.
func (a *AudioService) State() PlayerState {
	p := a.pl
	p.mu.Lock()
	st := PlayerState{
		Status:     string(p.status),
		TrackId:    p.trackId,
		PlaylistId: p.playlistId,
		ItemId:     p.itemId,
		DurationMs: p.durationMs,
		NextTitle:  p.nextTitle,
		Volume:     p.currentVolumeLocked(),
		DeviceName: p.deviceName,
	}
	devRate := p.devRate
	p.mu.Unlock()

	if devRate > 0 {
		st.PositionMs = int(p.pos.Load() * 1000 / int64(devRate))
	}
	// Пик читается и сбрасывается атомарно (Swap): следующее чтение должно
	// увидеть максимум только с этого момента, а не унаследовать старый.
	st.Peak = math.Float64frombits(p.peak.Swap(0))
	if st.DeviceName == "" {
		// Устройство ещё ни разу не открывалось в этой сессии (ленивое
		// открытие — только при первом Play): показываем то, что реально
		// будет открыто, не выдумывая имя ещё не запрошенного устройства.
		if p.selectedDevice() == "" {
			st.DeviceName = systemDefaultDeviceName
		} else {
			st.DeviceName = "Устройство ещё не открыто"
		}
	}
	st.DeviceLost = p.deviceLost.Load()
	return st
}

// trimmedDurationMs — «длительность после обрезки» для UI (план, этап 5):
// PositionMs/DurationMs и Seek() считаются от TrimStartMs, а не от начала
// исходного файла, поэтому это то, что реально услышит оператор, а не полная
// длина файла. TrimEndMs == 0 значит "играть до конца" (см. models.AudioTrack).
func trimmedDurationMs(track models.AudioTrack) int {
	end := track.DurationMs
	if track.TrimEndMs > 0 {
		end = track.TrimEndMs
	}
	d := end - track.TrimStartMs
	if d < 0 {
		return 0
	}
	return d
}

// trackChainBounds считает totalSamples/fadeSamples (СЭМПЛЫ УСТРОЙСТВА, см.
// deviceSamples в audio_player.go) для buildChain при СТАРТЕ трека
// (posSamples подразумевается 0 — пересчёт на другую позицию делает
// trimFadeSamplesAt, см. Seek/FadeOutStop). totalSamples == -1 означает "без
// ограничения": ни TrimStartMs, ни TrimEndMs, ни фейд не заданы — тогда
// buildChain строит чистую трубу без единого beep.Take, как было до этапа 5.
//
// fadeMs — FadeMs плейлиста, а не самого трека (фейд — граница между
// элементами плейлиста, план: "Fade-out перед автопереходом строится в
// цепочку сразу", а не по сигналу done — тот приходит, когда дренировать уже
// нечего). Обрезаем fadeSamples до totalSamples: иначе (короткий трим/трек
// короче фейда) beep.Take(totalSamples-fadeSamples, ...) получил бы
// отрицательную длину и фейд перекрыл бы саму границу обрезки, "съев" кусок
// сверх TrimEndMs.
func trackChainBounds(track models.AudioTrack, fadeMs int, devRate beep.SampleRate) (totalSamples, fadeSamples int) {
	totalSamples = -1
	if track.TrimStartMs > 0 || track.TrimEndMs > 0 || fadeMs > 0 {
		totalSamples = deviceSamples(trimmedDurationMs(track), devRate)
	}
	if fadeMs > 0 && totalSamples >= 0 {
		fadeSamples = deviceSamples(fadeMs, devRate)
		if fadeSamples > totalSamples {
			fadeSamples = totalSamples
		}
	}
	return totalSamples, fadeSamples
}

// openTrackSource открывает файл трека из медиатеки и сразу ставит курсор
// декодера на TrimStartMs. Общее начало двух путей старта трека — Play()
// (ниже) и подготовки следующего элемента автоперехода (decodePendingNext,
// audio_autoadvance.go): открывают они файл одинаково, и ловушку ниже
// достаточно держать в одном месте, а не помнить про неё в каждом.
//
// ⚠️ fileSamples, не deviceSamples: src.Seek() двигает курсор ДЕКОДЕРА,
// который читает файл на его РОДНОЙ частоте, — главная ловушка этапа 5.
//
// При любой ошибке src уже закрыт здесь: вызывающему остаётся вернуть err.
func openTrackSource(mediaDir string, track models.AudioTrack) (src beep.StreamSeekCloser, format beep.Format, trimStartFrame int, err error) {
	src, format, err = decodeExt(filepath.Join(mediaDir, track.FileName), filepath.Ext(track.FileName))
	if err != nil {
		return nil, beep.Format{}, 0, fmt.Errorf("не удалось открыть файл %q: %w", track.FileName, err)
	}
	trimStartFrame = fileSamples(track.TrimStartMs, format.SampleRate)
	if trimStartFrame > 0 {
		if err := src.Seek(trimStartFrame); err != nil {
			src.Close()
			return nil, beep.Format{}, 0, fmt.Errorf("не удалось обрезать начало %q: %w", track.FileName, err)
		}
	}
	return src, format, trimStartFrame, nil
}

// Play начинает воспроизведение элемента плейлиста itemId. Повторный Play на
// уже ЗАГРУЖЕННЫЙ itemId — no-op, не рестарт (И5, расширено): "загружен"
// значит играет ИЛИ на паузе — isCurrentItem, а не isPlayingItem. Раньше
// no-op проверялся только по "играет", и двойной клик/Enter по строке НА
// ПАУЗЕ перезапускал трек с нуля — кнопка play в строке вела себя иначе,
// потому что playItem во фронте (PlaylistPanel.svelte) отдельно ловит этот
// случай и зовёт Toggle() сама; Play() обязан быть безопасен и без этой
// подстраховки на фронте.
func (a *AudioService) Play(playlistIdF, itemIdF float32) error {
	itemId := uint(itemIdF)
	playlistId := uint(playlistIdF)

	if a.pl.isCurrentItem(itemId) {
		return nil
	}

	if err := a.pl.ensureDevice(); err != nil {
		return err
	}

	var item models.PlaylistItem
	if err := inits.DB.Preload("Track").First(&item, itemId).Error; err != nil {
		return fmt.Errorf("элемент плейлиста не найден: %w", err)
	}
	track := item.Track
	if track.FileName == "" {
		return fmt.Errorf("у трека %q нет файла", track.Title)
	}
	mediaDir, err := paths.MediaDir()
	if err != nil {
		return fmt.Errorf("не удалось определить каталог медиатеки: %w", err)
	}

	// FadeMs плейлиста нужен уже сейчас, а не по приходу done (этап 5,
	// buildChain): фейд-аут перед автопереходом строится в цепочку сразу при
	// старте трека. Отсутствие плейлиста (не должно случаться, но не повод
	// падать) — fadeMs=0, как если бы фейд не был настроен. Правило «фейд
	// только когда есть куда переходить» — в fadeMsBefore.
	fadeMs := 0
	var playlist models.Playlist
	if err := inits.DB.First(&playlist, playlistId).Error; err == nil {
		fadeMs = a.fadeMsBefore(playlist, itemId)
	}

	// Резервируем поколение ДО дорогой сборки цепочки (И4): любой
	// play/stop/seek, начавшийся, пока этот Play ещё декодирует файл,
	// должен суметь его отменить через несовпадение gen в startTrack.
	gen := a.pl.nextGen()

	src, format, trimStartFrame, err := openTrackSource(mediaDir, track)
	if err != nil {
		return err
	}

	devRate := a.pl.deviceRateSnapshot()
	totalSamples, fadeSamples := trackChainBounds(track, fadeMs, devRate)
	chain := buildChain(src, format.SampleRate, devRate, track.GainDb, totalSamples, fadeSamples, a.pl.done, gen)

	meta := trackMeta{
		trackId:            track.ID,
		playlistId:         playlistId,
		itemId:             itemId,
		durationMs:         trimmedDurationMs(track),
		fileRate:           format.SampleRate,
		gainDb:             track.GainDb,
		trimStartFileFrame: trimStartFrame,
		totalSamples:       totalSamples,
		fadeSamples:        fadeSamples,
	}
	installed, staleSrc := a.pl.startTrack(gen, chain, src, meta)
	if staleSrc != nil {
		staleSrc.Close() // файловый IO — вне p.mu (И2/И3), startTrack уже отпустил его
	}
	if !installed {
		return nil // обогнали более новым play/stop/seek — не ошибка (И4)
	}
	// Предыдущий заранее подготовленный "следующий" (для другого трека) уже
	// не актуален — этот Play() мог быть ручным вмешательством оператора, а
	// не автопереходом. preparePendingNext ниже и сам корректно заменил бы
	// устаревший pending (afterItemId у него другой), но дропаем явно —
	// читаемее на месте вызова и не оставляет на дольше, чем нужно, шанс
	// увидеть в State().NextTitle заголовок чужого трека.
	a.dropPending()
	a.emit("audio_track_changed", a.State())
	// Этап 4: если playlistId принадлежит плейлисту с AutoAdvance, заранее
	// открыть следующий элемент — decodeExt дорог (И3), и делать это по
	// приходу done означало бы дыру между треками (см. audio_autoadvance.go).
	a.preparePendingNext(playlistId, itemId)
	return nil
}

// Toggle переключает паузу текущего трека. No-op, если ничего не играет.
func (a *AudioService) Toggle() {
	a.pl.toggle()
}

// Stop останавливает воспроизведение без фейда (фейд — этап 5). Устройство
// не закрывается: следующий Play переиспользует уже открытое — дешевле, чем
// поднимать WASAPI заново на каждый клик.
func (a *AudioService) Stop() {
	a.dropPending()
	oldSrc := a.pl.stop()
	if oldSrc != nil {
		oldSrc.Close() // файловый IO — вне p.mu (И2/И3)
	}
	a.emit("audio_stopped", nil)
}

// Seek перематывает текущий трек. ms — позиция ОТНОСИТЕЛЬНО TrimStartMs
// (0 = начало прослушиваемого окна, см. trimmedDurationMs: "PositionMs/
// DurationMs считаются от TrimStartMs, а не от начала исходного файла") —
// поэтому к файловому Seek ниже прибавляется trimStartFileFrame, а к позиции
// устройства (deviceFrame, для p.pos и для trimFadeSamplesAt) — нет: та уже в
// системе координат "от начала прослушивания".
//
// snapshotForSeek атомарно (одной критической секцией под p.mu) резервирует
// поколение И отцепляет текущую цепочку от микшера — decode.Seek() ниже
// вызывается вне p.mu (дёшево: go-mp3.Seek — это lseek по уже построенной в
// конструкторе таблице фреймов плюс decode одного фрейма, а не полный проход
// по файлу), но декодер уже недостижим для data-колбэка, пока мы им
// распоряжаемся (И1). Ресемплер и громкость пересобираются заново поверх
// того же декодера (buildChain) — иначе первые доли секунды после перемотки
// звучал бы хвост старого места (beep.Resampler не сбрасывается,
// resample.go:78-86). По той же причине пересборка НИКОГДА не продолжает
// фейд, что бы ни играло на месте seek: buildChain всегда строит свежий,
// независимый effects.Transition, отсчитывающий прогресс от pos=0 —
// попытка "продолжить" старый Transition через новый объект вместо этого
// заставила бы гейн начаться заново со startGain при каждой перемотке.
func (a *AudioService) Seek(msF float32) error {
	ms := int(msF)
	if ms < 0 {
		ms = 0
	}

	snap, gen, ok := a.pl.snapshotForSeek()
	if !ok {
		return fmt.Errorf("нечего перематывать: ничего не играет")
	}

	fileFrame := fileSamples(ms, snap.fileRate) + snap.trimStartFileFrame
	if err := snap.src.Seek(fileFrame); err != nil {
		// snapshotForSeek уже отцепил цепочку от микшера и зарезервировал
		// gen — если оставить это как есть, движок навсегда останется в
		// псевдо-играющем состоянии (см. abandonFailedSeek). Известный
		// триггер: beep/flac отказывает в Seek на FLAC без seek-таблицы (наш
		// собственный формат конвертации при импорте), vorbis — на любом
		// отказе SetPosition, mp3 — на позиции за пределами файла.
		a.dropPending() // "следующий" трек автоперехода больше не актуален — трек оборван, не доиграет
		if oldSrc := a.pl.abandonFailedSeek(gen, snap.src); oldSrc != nil {
			oldSrc.Close() // файловый IO — вне p.mu (И2/И3)
			a.emit("audio_stopped", nil)
		}
		return fmt.Errorf("не удалось перемотать: %w", err)
	}

	deviceFrame := deviceSamples(ms, snap.devRate)
	// Take не идемпотентен (compositors.go:20-23) — remaining/fade считаются
	// ЗАНОВО от новой позиции, а не от исходного snap.totalSamples (план,
	// этап 5, главная ловушка Take).
	remaining, fade := trimFadeSamplesAt(snap.totalSamples, snap.fadeSamples, deviceFrame)
	chain := buildChain(snap.src, snap.fileRate, snap.devRate, snap.gainDb, remaining, fade, a.pl.done, gen)

	// resumeChain==false значит нас обогнал Stop()/другой Play()/Seek() —
	// не ошибка (И4): та операция уже поставила плеер в правильное
	// состояние, наш устаревший результат просто отбрасывается.
	a.pl.resumeChain(gen, chain, snap.src, int64(deviceFrame))
	return nil
}

// FadeOutStop останавливает воспроизведение с фейдом — «Стоп с фейдом»
// (план, этап 5). Использует FadeMs плейлиста ТЕКУЩЕГО трека, прочитанный
// заново из БД (а не кэш trackMeta): оператор мог поправить FadeMs уже после
// того, как трек начал играть, и стоп должен уважать актуальное значение.
//
// FadeMs<=0 сводится к обычному Stop(): effects.Transition вообще не
// заводится (см. buildChain про NaN при len==0) — реализовано без
// дублирования логики самим вызовом Stop().
//
// В остальном это Seek-подобная пересборка (snapshotForSeek), а не p.stop():
// текущая цепочка заменяется на Take(fadeSamples, Transition(...,1,0,...))
// ОТ ТЕКУЩЕЙ позиции — totalSamples и fadeSamples для buildChain здесь
// совпадают: фейд — это ВСЯ оставшаяся жизнь цепочки, ничего не играет после
// её конца. Когда она (гарантированно, за счёт внешнего beep.Take —
// см. buildChain) дренируется, done придёт как обычное "трек доиграл", и
// watchPlayerEvents обработает его как обычный конец воспроизведения:
// dropPending здесь — по аналогии со Stop(), заранее подготовленный
// "следующий" трек (автопереход) больше не актуален, раз оператор явно
// остановил воспроизведение.
//
// ⚠️ Два ранних выхода на жёсткий Stop(), ДО построения фейд-цепочки:
//   - p.fadingOut уже true — второй "Стоп с фейдом" подряд. Повторная
//     пересборка ниже начала бы НЕЗАВИСИМЫЙ effects.Transition с нуля
//     (buildChain/resumeChain не умеют "продолжить" старый), то есть на
//     мгновение подняла бы громкость обратно к startGain=1 и запустила фейд
//     заново — оператор, дважды нажавший стоп, хочет тишины сейчас, а не
//     повторного фейда.
//   - трек на паузе — цепочка фейда, поставленная поверх приостановленного
//     beep.Ctrl, никогда не продвинется (Ctrl.Paused пропускает вызов
//     обёрнутого Stream целиком): done не придёт, оператор навсегда
//     останется на паузе с "фейднутым" в памяти треком.
func (a *AudioService) FadeOutStop() {
	if a.pl.fadingOut.Load() {
		a.Stop()
		return
	}

	a.dropPending()

	playlistId, _, ok := a.pl.currentItem()
	if !ok {
		a.Stop()
		return
	}
	if a.pl.isPaused() {
		a.Stop()
		return
	}

	fadeMs := 0
	var playlist models.Playlist
	if err := inits.DB.First(&playlist, playlistId).Error; err == nil {
		fadeMs = playlist.FadeMs
	}
	if fadeMs <= 0 {
		a.Stop()
		return
	}

	pos := a.pl.pos.Load() // сохраняем прогресс на время фейда — не перемотка, не 0
	snap, gen, ok := a.pl.snapshotForSeek()
	if !ok {
		// Трек успел закончиться сам между currentItem() и этим моментом —
		// гонка безопасна (И4): просто сообщаем "стоп".
		a.emit("audio_stopped", nil)
		return
	}

	fadeSamples := deviceSamples(fadeMs, snap.devRate)
	chain := buildChain(snap.src, snap.fileRate, snap.devRate, snap.gainDb, fadeSamples, fadeSamples, a.pl.done, gen)
	if a.pl.resumeChain(gen, chain, snap.src, pos) {
		a.pl.fadingOut.Store(true)
	}
}

// SetVolume меняет общую громкость (0..1) немедленно и планирует
// отложенную запись в GlobalState.AudioVolume — по образцу
// scheduleWindowGeometrySave в db_outputs.go: слайдер не должен бить SQLite
// на каждое движение.
func (a *AudioService) SetVolume(v float64) {
	// Клампим один раз здесь, а не только внутри setVolumeLocked: тот
	// клампит для себя (и для newPlayer), но в замыкание ниже уходил бы
	// исходный, неклампленный v — в GlobalState сохранилось бы, скажем,
	// -0.3 вместо честного 0.
	if v < 0 {
		v = 0
	} else if v > 1 {
		v = 1
	}

	a.pl.mu.Lock()
	a.pl.setVolumeLocked(v)
	a.pl.mu.Unlock()

	const debounce = 400 * time.Millisecond
	a.volSaveMu.Lock()
	defer a.volSaveMu.Unlock()
	if a.volSaveTimer != nil {
		a.volSaveTimer.Stop()
	}
	a.volSaveTimer = time.AfterFunc(debounce, func() {
		if err := inits.DB.Model(&models.GlobalState{}).Where("id = ?", 1).Update("audio_volume", v).Error; err != nil {
			log.Println("SetVolume: error saving volume", err)
		}
	})
}
