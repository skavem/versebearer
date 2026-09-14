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

// Медиатека фонограмм: чтение списка, правка метаданных, удаление. Отделено
// от audio_service.go, где остались жизненный цикл сервиса и транспорт
// (Play/Seek/FadeOutStop/громкость): к движку эти три метода обращаются
// только чтобы остановить или инвалидировать то, что сейчас звучит, а в
// остальном это обычная работа с БД и файлами, как в dbHandler.go.
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

	// a.pl может быть nil в тестах, которые конструируют AudioService{} без
	// NewAudioService (им движок не нужен) — RemoveTrack тогда просто
	// пропускает шаг остановки, звука ни в одном таком тесте не бывает.
	stopped := false
	var pendingPlaylistId, pendingAfterItemId uint
	pendingDropped := false
	if a.pl != nil {
		if a.pl.isCurrentTrack(id) {
			a.Stop()
			stopped = true
		}
		// Трек мог быть не текущим, а уже заранее открытым "следующим"
		// (этап 4, автопереход) — тот же sharing-violation риск на Windows,
		// только для decodeExt внутри fillPendingNext, а не для os.Remove
		// ниже. Дропаем ДО удаления файла/строки (чтобы декодер точно
		// закрылся раньше os.Remove), пересчитываем — ПОСЛЕ (когда БД уже не
		// содержит удалённых ссылок, см. deleteTrackIfUnused).
		pendingPlaylistId, pendingAfterItemId, pendingDropped = a.dropPendingForTrack(id)
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

	// Стоп (isCurrentTrack) уже обнулил pending целиком через a.Stop() ->
	// dropPending — нечего пересчитывать. Иначе, если удаляемый трек был
	// pending каким-то другим, всё ещё играющим элементом, — пересчитываем,
	// чтобы автопереход не остался без пары (см. audio_autoadvance.go).
	if pendingDropped && !stopped {
		a.refreshPending(pendingPlaylistId, pendingAfterItemId)
	}

	a.emit("audio_tracks_update", a.ListTracks())
	return nil
}
