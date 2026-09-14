package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/paths"
)

// Медиатека фонограмм: чтение списка, правка метаданных, удаление трека
// (явное — RemoveTrack, и по исчезновению последней ссылки —
// deleteTrackIfUnused, которую зовут мутации плейлиста). Отделено от
// audio_service.go, где остались жизненный цикл сервиса и транспорт
// (Play/Seek/FadeOutStop/громкость): к движку эти методы обращаются только
// чтобы остановить или инвалидировать то, что сейчас звучит, а в остальном
// это обычная работа с БД и файлами, как в dbHandler.go.
// Импорт новых файлов — отдельно, в audio_import.go.

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

	var track models.AudioTrack
	if err := inits.DB.First(&track, id).Error; err != nil {
		return nil, fmt.Errorf("трек не найден: %w", err)
	}

	updates := map[string]any{}
	if input.Title != nil {
		updates["title"] = *input.Title
	}
	if input.Artist != nil {
		updates["artist"] = *input.Artist
	}
	// trimStart/trimEnd — итоговые значения ПОСЛЕ применения патча (частичный
	// TrackInput мог прислать только одно из двух полей): валидировать нужно
	// результат, а не изменённое поле само по себе, иначе патч, меняющий
	// только TrimStartMs, мог бы молча образовать невалидную пару со старым
	// TrimEndMs.
	trimStart, trimEnd := track.TrimStartMs, track.TrimEndMs
	if input.TrimStartMs != nil {
		trimStart = *input.TrimStartMs
		updates["trim_start_ms"] = trimStart
	}
	if input.TrimEndMs != nil {
		trimEnd = *input.TrimEndMs
		updates["trim_end_ms"] = trimEnd
	}
	// TrimEndMs==0 значит "до конца файла" (models.AudioTrack) — границы нет,
	// проверять нечего. Иначе конец обязан быть строго после начала: end<=start
	// даёт trimmedDurationMs()==0 -> beep.Take(0) -> трек не звучит и мгновенно
	// отдаёт done — с AutoAdvance+Loop плейлист пролетит по кругу за секунду.
	// Источник истины — здесь (защищает от любого вызывающего); фронт
	// (EditTrackModal) дублирует ту же проверку только для мгновенной
	// обратной связи до сохранения.
	if trimEnd > 0 && trimEnd <= trimStart {
		return nil, fmt.Errorf("конец обрезки должен быть позже начала")
	}
	if input.GainDb != nil {
		updates["gain_db"] = *input.GainDb
	}
	// decodingAffected — правка trim/gain могла сделать неверным уже
	// подготовленный "следующий" трек автоперехода (этап 4): pending
	// декодирован и посчитан (totalSamples/fadeSamples/trimStartFileFrame,
	// gainDb) со старыми значениями — без пересчёта автопереход поставил бы
	// устаревшую версию правки. Title/Artist на декодирование не влияют
	// вообще — трогать pending ради них не нужно (лишний decodeExt, И3).
	decodingAffected := input.TrimStartMs != nil || input.TrimEndMs != nil || input.GainDb != nil
	if len(updates) > 0 {
		if err := inits.DB.Model(&models.AudioTrack{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return nil, fmt.Errorf("не удалось сохранить изменения: %w", err)
		}
	}
	if err := inits.DB.First(&track, id).Error; err != nil {
		return nil, fmt.Errorf("трек не найден: %w", err)
	}

	// dropPendingForTrack сначала закрывает старый декодер (сохраняет
	// playlistId/afterItemId, на который он указывал), а refreshPending
	// заново открывает его УЖЕ по только что сохранённым trim/gain — так
	// "следующий" трек переоткрывается со свежими значениями, а не просто
	// исчезает до естественного следующего done (что дало бы дырку в
	// автопереходе — тот самый класс бага, что и в audio_autoadvance.go).
	if a.pl != nil && decodingAffected {
		if playlistId, afterItemId, ok := a.dropPendingForTrack(id); ok {
			a.refreshPending(playlistId, afterItemId)
		}
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

	hold := a.releaseTrackFile(id)

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

	a.refreshPendingAfterRelease(hold)
	a.emit("audio_tracks_update", a.ListTracks())
	return nil
}

// trackFileHold — что пришлось остановить и закрыть, чтобы файл трека можно
// было удалить (releaseTrackFile), и чей автопереход из-за этого остался без
// пары (refreshPendingAfterRelease). Две половины разнесены во времени
// намеренно: закрывать декодеры нужно ДО os.Remove, а пересчитывать
// "следующий" — ПОСЛЕ удаления строк из БД, иначе refreshPending ответит по
// данным, которые ещё содержат удаляемое.
type trackFileHold struct {
	stopped        bool
	pendingDropped bool
	playlistId     uint
	afterItemId    uint
}

// releaseTrackFile закрывает всё, что держит файл трека открытым: играющий
// декодер (если звучит именно он) и заранее подготовленный "следующий"
// (этап 4, автопереход). Обязательный пролог любого удаления трека —
// os.Remove на Windows падает с sharing violation, пока файл держит открытый
// декодер, а ошибка эта тихая: строка из БД уйдёт, файл останется сиротой
// навсегда.
//
// a.pl может быть nil в тестах, которые конструируют AudioService{} без
// NewAudioService (им движок не нужен) — тогда освобождать нечего, звука ни в
// одном таком тесте не бывает.
func (a *AudioService) releaseTrackFile(trackId uint) trackFileHold {
	if a.pl == nil {
		return trackFileHold{}
	}
	var hold trackFileHold
	if a.pl.isCurrentTrack(trackId) {
		a.Stop()
		hold.stopped = true
	}
	hold.playlistId, hold.afterItemId, hold.pendingDropped = a.dropPendingForTrack(trackId)
	return hold
}

// refreshPendingAfterRelease — эпилог удаления трека, парный к
// releaseTrackFile: вызывать ПОСЛЕ того, как удаление отражено в БД.
//
// Стоп уже сам обнулил pending целиком (a.Stop -> dropPending) — тогда
// пересчитывать нечего. Иначе, если удалённый трек был заранее открытым
// "следующим" для какого-то другого, всё ещё играющего элемента, —
// пересчитываем, чтобы автопереход не остался без пары и не оборвал эфир
// тишиной (см. audio_autoadvance.go).
func (a *AudioService) refreshPendingAfterRelease(hold trackFileHold) {
	if hold.pendingDropped && !hold.stopped {
		a.refreshPending(hold.playlistId, hold.afterItemId)
	}
}

// deleteUnusedTracksFor зовёт deleteTrackIfUnused по каждому УНИКАЛЬНОМУ
// TrackId из items — общий хвост RemovePlaylist и ClearPlaylist: два
// элемента одного плейлиста иногда ссылаются на один и тот же трек (тот же
// файл добавили дважды). deleteTrackIfUnused и так безопасен на повторный
// вызов (второй раз count уже 0, либо First не находит строку и тихо
// возвращается), но не звать его дважды на один trackId дешевле.
func (a *AudioService) deleteUnusedTracksFor(items []models.PlaylistItem) {
	seen := make(map[uint]bool, len(items))
	for _, it := range items {
		if seen[it.TrackId] {
			continue
		}
		seen[it.TrackId] = true
		a.deleteTrackIfUnused(it.TrackId)
	}
}

// deleteTrackIfUnused удаляет трек из медиатеки (файл + строка), если на
// него не осталось ни одной ссылки (PlaylistItem.TrackId) ни в одном
// плейлисте. Общий хвост RemoveFromPlaylist и ClearPlaylist — правка 1:
// «удаление элемента из плейлиста удаляет трек и файл с диска, ЕСЛИ трек
// больше не используется ни в одном плейлисте».
//
// ⚠️ Это единственное место, где решается "остановить или доиграть" для
// удаляемого трека — и решение основано ИСКЛЮЧИТЕЛЬНО на физике: os.Remove
// на Windows падает с sharing violation, пока файл держит открытый декодер.
// Трек ещё используется в другом плейлисте (count>0, ранний возврат ниже) —
// файл не удаляется, значит и стоп не нужен: если этот же трек играет через
// какой-то другой элемент прямо сейчас, он спокойно доигрывает до конца,
// просто не унаследует автопереход (см. tryAdvance/refreshPending).
// Единственная ссылка — файл будет стёрт с диска, и тогда остановка
// обязательна, если трек именно сейчас звучит (releaseTrackFile).
func (a *AudioService) deleteTrackIfUnused(trackId uint) {
	var count int64
	if err := inits.DB.Model(&models.PlaylistItem{}).Where("track_id = ?", trackId).Count(&count).Error; err != nil {
		log.Println("deleteTrackIfUnused: error counting references", err)
		// MEDIUM №7 обзора: без этого сигнала ошибка подсчёта ссылок молча
		// давала ранний return — трек не чистился вообще, а вызывающий
		// (RemoveFromPlaylist/ClearPlaylist/RemovePlaylist) уже отчитался
		// оператору об успехе. production собран с -H windowsgui — один
		// log.Println здесь никто не увидит.
		a.emit("audio_error", fmt.Sprintf("не удалось проверить ссылки на трек: %s", err.Error()))
		return
	}
	if count > 0 {
		return
	}

	var track models.AudioTrack
	if err := inits.DB.First(&track, trackId).Error; err != nil {
		return // уже удалён кем-то ещё — нечего делать
	}

	hold := a.releaseTrackFile(trackId)

	if mediaDir, err := paths.MediaDir(); err == nil && track.FileName != "" {
		// Отказ os.Remove — осознанно тихий (только лог, без audio_error,
		// MEDIUM №7 обзора): sharing-violation на Windows решается ДО этого
		// места (releaseTrackFile выше уже закрыл все декодеры, которые
		// могли держать файл), а строка AudioTrack всё равно удаляется ниже
		// — трек уйдёт из UI, и оператору сигналить уже не о чем действенном
		// (RemoveTrack придерживается того же: "файл-сирота на диске
		// безопаснее фантомной записи в медиатеке").
		if err := os.Remove(filepath.Join(mediaDir, track.FileName)); err != nil && !os.IsNotExist(err) {
			log.Println("deleteTrackIfUnused: error deleting file", err)
		}
	}

	if err := inits.DB.Delete(&models.AudioTrack{}, trackId).Error; err != nil {
		log.Println("deleteTrackIfUnused: error deleting row", err)
		a.emit("audio_error", fmt.Sprintf("не удалось удалить трек из медиатеки: %s", err.Error()))
		return
	}

	a.refreshPendingAfterRelease(hold)
	a.emit("audio_tracks_update", a.ListTracks())
}
