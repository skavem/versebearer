package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/paths"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// AudioService — Wails-сервис вкладки «Звук». На этом этапе (1) в нём нет
// поля движка (`pl *player`): само воспроизведение появляется в этапе 2.
// Заводить поле заранее означало бы либо оставлять его nil без единого
// использования, либо тянуть в этот коммит код движка, которого по плану ещё
// не должно быть.
type AudioService struct {
	app *application.App
}

func NewAudioService() *AudioService {
	return &AudioService{}
}

// emit повторяет обёртку DbHandler.emit: до присваивания app (и в тестах)
// события молча глотаются, а не роняют вызов.
func (a *AudioService) emit(name string, data any) {
	if a.app != nil {
		a.app.Event.Emit(name, data)
	}
}

// ServiceShutdown, а не Close: Wails исключает из биндингов только
// ServiceName/ServiceStartup/ServiceShutdown/ServeHTTP — экспортированный
// Close() уехал бы во фронт как кнопка «разломать звук». Пока движка нет,
// закрывать нечего; на этапе 2 сюда переезжает остановка устройства.
func (a *AudioService) ServiceShutdown() error {
	return nil
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

// RemoveTrack удаляет трек: сперва ссылки на него из плейлистов (плейлистов
// в этом этапе ещё не завести из UI, но модель уже есть — не оставлять
// висячих PlaylistItem, если они появятся), затем файл, затем строку.
//
// ⚠️ os.Remove на Windows падает с ошибкой sharing violation, пока файл
// держит открытый декодер. На этом этапе воспроизведения ещё нет, но
// комментарий остаётся здесь заранее: на этапе 2 перед удалением файла
// понадобится синхронно дождаться Stop()/src.Close() играющего трека — иначе
// удаление той же фонограммы, что сейчас звучит, будет молча проваливаться.
// Если os.Remove всё же не удался, строку удаляем всё равно: файл-сирота на
// диске безопаснее фантомной записи в медиатеке, а имя по хешу означает, что
// повторный импорт того же файла его переиспользует.
func (a *AudioService) RemoveTrack(idF float32) error {
	id := uint(idF)
	var track models.AudioTrack
	if err := inits.DB.First(&track, id).Error; err != nil {
		return err
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
