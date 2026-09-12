package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/paths"

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

	// quit останавливает watchPlayerEvents при ServiceShutdown. Закрывается
	// ровно один раз; done/lost самих закрывать нельзя — data-колбэк malgo
	// может успеть отправить в них до Uninit устройства.
	quit chan struct{}
}

func NewAudioService() *AudioService {
	// Громкость читаем из GlobalState сразу: inits.DB готова к этому
	// моменту (открывается в package init(), до main()). Ошибка чтения —
	// не повод падать, откатываемся на полную громкость, как и сама
	// колонка по умолчанию (models.GlobalState.AudioVolume `gorm:"default:1"`).
	vol := 1.0
	var gs models.GlobalState
	if err := inits.DB.First(&gs, 1).Error; err == nil {
		vol = gs.AudioVolume
	}
	a := &AudioService{pl: newPlayer(vol), quit: make(chan struct{})}
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
// пропало" (lost, этап 3 использует его полнее — здесь пока просто честно
// сообщаем оператору и переходим в idle, а не оставляем UI показывать
// "играет" вечно). Оба канала обслуживаются одной горутиной, а не data-
// колбэком malgo (И2: там нельзя ни p.mu, ни emit синхронно). quit
// останавливает горутину при ServiceShutdown, чтобы она не дёргала emit
// после разборки приложения.
func (a *AudioService) watchPlayerEvents() {
	for {
		select {
		case gen := <-a.pl.done:
			if oldSrc, changed := a.pl.finishIfCurrent(gen); changed {
				if oldSrc != nil {
					oldSrc.Close() // файловый IO — вне p.mu (И2/И3), мы уже не под ним
				}
				a.emit("audio_stopped", nil)
			}
		case <-a.pl.lost:
			oldSrc := a.pl.forceIdleOnDeviceLost()
			if oldSrc != nil {
				oldSrc.Close()
			}
			a.emit("audio_error", "устройство вывода пропало")
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
	}
	devRate := p.devRate
	p.mu.Unlock()

	if devRate > 0 {
		st.PositionMs = int(p.pos.Load() * 1000 / int64(devRate))
	}
	// Пик читается и сбрасывается атомарно (Swap): следующее чтение должно
	// увидеть максимум только с этого момента, а не унаследовать старый.
	st.Peak = math.Float64frombits(p.peak.Swap(0))
	// Выбор устройства — этап 3; пока плеер всегда играет в системное по
	// умолчанию.
	st.DeviceName = "Системное по умолчанию"
	st.DeviceLost = false
	return st
}

// Play начинает воспроизведение элемента плейлиста itemId. Повторный Play
// на уже играющий itemId — no-op, не рестарт (И5): оператор, дважды
// кликнувший по строке, не должен услышать, как трек начался заново.
func (a *AudioService) Play(playlistIdF, itemIdF float32) error {
	itemId := uint(itemIdF)
	playlistId := uint(playlistIdF)

	if a.pl.isPlayingItem(itemId) {
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

	// Резервируем поколение ДО дорогой сборки цепочки (И4): любой
	// play/stop/seek, начавшийся, пока этот Play ещё декодирует файл,
	// должен суметь его отменить через несовпадение gen в startTrack.
	gen := a.pl.nextGen()

	src, format, err := decodeExt(filepath.Join(mediaDir, track.FileName), filepath.Ext(track.FileName))
	if err != nil {
		return fmt.Errorf("не удалось открыть файл %q: %w", track.FileName, err)
	}

	devRate := a.pl.deviceRateSnapshot()
	chain := buildChain(src, format.SampleRate, devRate, track.GainDb, a.pl.done, gen)

	meta := trackMeta{
		trackId:    track.ID,
		playlistId: playlistId,
		itemId:     itemId,
		durationMs: track.DurationMs, // трим ещё не применяется — этап 5
		fileRate:   format.SampleRate,
		gainDb:     track.GainDb,
	}
	installed, staleSrc := a.pl.startTrack(gen, chain, src, meta)
	if staleSrc != nil {
		staleSrc.Close() // файловый IO — вне p.mu (И2/И3), startTrack уже отпустил его
	}
	if !installed {
		return nil // обогнали более новым play/stop/seek — не ошибка (И4)
	}
	a.emit("audio_track_changed", a.State())
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
	oldSrc := a.pl.stop()
	if oldSrc != nil {
		oldSrc.Close() // файловый IO — вне p.mu (И2/И3)
	}
	a.emit("audio_stopped", nil)
}

// Seek перематывает текущий трек. snapshotForSeek атомарно (одной
// критической секцией под p.mu) резервирует поколение И отцепляет текущую
// цепочку от микшера — decode.Seek() ниже вызывается вне p.mu (дёшево:
// go-mp3.Seek — это lseek по уже построенной в конструкторе таблице
// фреймов плюс decode одного фрейма, а не полный проход по файлу), но
// декодер уже недостижим для data-колбэка, пока мы им распоряжаемся (И1).
// Ресемплер и громкость пересобираются заново поверх того же декодера
// (buildChain) — иначе первые доли секунды после перемотки звучал бы
// хвост старого места (beep.Resampler не сбрасывается, resample.go:78-86).
func (a *AudioService) Seek(msF float32) error {
	ms := int(msF)
	if ms < 0 {
		ms = 0
	}

	snap, gen, ok := a.pl.snapshotForSeek()
	if !ok {
		return fmt.Errorf("нечего перематывать: ничего не играет")
	}

	fileFrame := int(float64(ms) / 1000 * float64(snap.fileRate))
	if err := snap.src.Seek(fileFrame); err != nil {
		return fmt.Errorf("не удалось перемотать: %w", err)
	}

	chain := buildChain(snap.src, snap.fileRate, snap.devRate, snap.gainDb, a.pl.done, gen)

	deviceFrame := int64(float64(ms) / 1000 * float64(snap.devRate))
	// resumeChain==false значит нас обогнал Stop()/другой Play()/Seek() —
	// не ошибка (И4): та операция уже поставила плеер в правильное
	// состояние, наш устаревший результат просто отбрасывается.
	a.pl.resumeChain(gen, chain, snap.src, deviceFrame)
	return nil
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

// ListTracks возвращает медиатеку фонограмм. Ошибку БД не глотает молча:
// без события «пустая медиатека» из-за сбоя чтения выглядела бы для
// оператора неотличимо от действительно пустой — с 40 треками он решил бы,
// что всё удалилось, и начал бы импортировать их заново.
func (a *AudioService) ListTracks() []models.AudioTrack {
	var tracks []models.AudioTrack
	if err := inits.DB.Order("id ASC").Find(&tracks).Error; err != nil {
		log.Println("ListTracks: error", err)
		a.emit("audio_error", fmt.Sprintf("не удалось прочитать медиатеку: %s", err.Error()))
		return nil
	}
	return tracks
}

// TrackInput — поля указателями: незаданное поле остаётся нетронутым, как
// StyleInput в dbHandler.go.
type TrackInput struct {
	Title       *string  `json:"title"`
	Artist      *string  `json:"artist"`
	TrimStartMs *int     `json:"trimStartMs"`
	TrimEndMs   *int     `json:"trimEndMs"`
	GainDb      *float64 `json:"gainDb"`
}

// UpdateTrack правит метаданные трека (название, исполнитель, trim, ручная
// поправка громкости) и возвращает обновлённую запись. Ошибка записи
// возвращается вызывающему явно — раньше она уходила только в log.Println, и
// оператор видел молчаливый откат правки в UI без единого слова объяснения.
func (a *AudioService) UpdateTrack(idF float32, input TrackInput) (*models.AudioTrack, error) {
	id := uint(idF)
	updates := map[string]any{}
	if input.Title != nil {
		updates["title"] = *input.Title
	}
	if input.Artist != nil {
		updates["artist"] = *input.Artist
	}
	if input.TrimStartMs != nil {
		updates["trim_start_ms"] = *input.TrimStartMs
	}
	if input.TrimEndMs != nil {
		updates["trim_end_ms"] = *input.TrimEndMs
	}
	if input.GainDb != nil {
		updates["gain_db"] = *input.GainDb
	}
	if len(updates) > 0 {
		if err := inits.DB.Model(&models.AudioTrack{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("не удалось сохранить изменения: %w", err)
		}
	}
	var track models.AudioTrack
	if err := inits.DB.First(&track, id).Error; err != nil {
		return nil, fmt.Errorf("трек не найден: %w", err)
	}
	a.emit("audio_tracks_update", a.ListTracks())
	return &track, nil
}

// RemoveTrack удаляет трек: сперва останавливает воспроизведение, если это
// именно он сейчас загружен (играет или на паузе), затем ссылки на него из
// плейлистов, затем файл, затем строку.
//
// ⚠️ os.Remove на Windows падает с ошибкой sharing violation, пока файл
// держит открытый декодер — поэтому Stop() здесь синхронный (дожидается
// src.Close()), а не просто сигнал. Без этого: os.Remove молча не удался бы
// (см. log.Println ниже — не виден в production, build/AGENTS.md:32), строка
// всё равно удалилась бы из БД, а трек продолжал бы звучать — и остановить
// его было бы уже нечем, в медиатеке его больше нет.
// Если os.Remove всё же не удался, строку удаляем всё равно: файл-сирота на
// диске безопаснее фантомной записи в медиатеке, а имя по хешу означает, что
// повторный импорт того же файла его переиспользует.
func (a *AudioService) RemoveTrack(idF float32) error {
	id := uint(idF)
	var track models.AudioTrack
	if err := inits.DB.First(&track, id).Error; err != nil {
		return err
	}

	// a.pl может быть nil в тестах, которые конструируют AudioService{} без
	// NewAudioService (им движок не нужен) — RemoveTrack тогда просто
	// пропускает шаг остановки, звука ни в одном таком тесте не бывает.
	if a.pl != nil && a.pl.isCurrentTrack(id) {
		a.Stop()
	}

	if err := inits.DB.Where("track_id = ?", id).Delete(&models.PlaylistItem{}).Error; err != nil {
		log.Println("RemoveTrack: error clearing playlist items", err)
	}

	if mediaDir, err := paths.MediaDir(); err == nil && track.FileName != "" {
		if err := os.Remove(filepath.Join(mediaDir, track.FileName)); err != nil && !os.IsNotExist(err) {
			log.Println("RemoveTrack: error deleting file", err)
		}
	}

	if err := inits.DB.Delete(&models.AudioTrack{}, id).Error; err != nil {
		return err
	}

	a.emit("audio_tracks_update", a.ListTracks())
	return nil
}
