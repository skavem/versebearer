package main

import (
	"fmt"
	"log"

	"changeme/backend/inits"
	"changeme/backend/models"

	"gorm.io/gorm"
)

// PlaylistFlagsInput — поля указателями, как TrackInput/StyleInput: незаданное
// поле остаётся нетронутым.
type PlaylistFlagsInput struct {
	AutoAdvance *bool `json:"autoAdvance"`
	Loop        *bool `json:"loop"`
	FadeMs      *int  `json:"fadeMs"`
}

// orderedItems — общий Preload для чтения плейлиста с элементами в порядке
// Position ASC. GORM без явного Order отдал бы элементы в порядке их
// собственного id (порядке создания), а не в порядке, который выставил
// ReorderPlaylist.
func orderedItems(db *gorm.DB) *gorm.DB {
	return db.Order("position ASC")
}

// ListPlaylists возвращает все плейлисты с элементами (и треками для них),
// упорядоченными по позиции. Ошибку БД не глотает молча — по тем же
// соображениям, что и ListTracks: пустой список из-за сбоя чтения не должен
// выглядеть для оператора неотличимо от действительно пустого.
func (a *AudioService) ListPlaylists() []models.Playlist {
	var playlists []models.Playlist
	if err := inits.DB.Preload("Items", orderedItems).Preload("Items.Track").
		Order("id ASC").Find(&playlists).Error; err != nil {
		log.Println("ListPlaylists: error", err)
		a.emit("audio_error", fmt.Sprintf("не удалось прочитать плейлисты: %s", err.Error()))
		return nil
	}
	return playlists
}

// playlistWithItems — как ListPlaylists, но один плейлист по id. Общий
// возврат для мутаций, которым план предписывает отдавать *models.Playlist,
// а не полный список.
func (a *AudioService) playlistWithItems(id uint) *models.Playlist {
	var playlist models.Playlist
	if err := inits.DB.Preload("Items", orderedItems).Preload("Items.Track").
		First(&playlist, id).Error; err != nil {
		log.Println("playlistWithItems: error", err)
		return nil
	}
	return &playlist
}

func (a *AudioService) CreatePlaylist(name string) *models.Playlist {
	playlist := models.Playlist{Name: name}
	if err := inits.DB.Create(&playlist).Error; err != nil {
		log.Println("CreatePlaylist: error", err)
		a.emit("audio_error", fmt.Sprintf("не удалось создать плейлист: %s", err.Error()))
		return nil
	}
	a.emit("audio_playlists_update", a.ListPlaylists())
	return a.playlistWithItems(playlist.ID)
}

func (a *AudioService) RenamePlaylist(idF float32, name string) []models.Playlist {
	id := uint(idF)
	if err := inits.DB.Model(&models.Playlist{}).Where("id = ?", id).Update("name", name).Error; err != nil {
		log.Println("RenamePlaylist: error", err)
		a.emit("audio_error", fmt.Sprintf("не удалось переименовать плейлист: %s", err.Error()))
	}
	playlists := a.ListPlaylists()
	a.emit("audio_playlists_update", playlists)
	return playlists
}

// RemovePlaylist удаляет плейлист и все его элементы. Единое правило (план,
// этап 4): если сейчас играет (или на паузе) что-то из этого плейлиста —
// воспроизведение останавливается первым делом, иначе горутина автоперехода
// станет искать следующий элемент среди мягко удалённых строк.
func (a *AudioService) RemovePlaylist(idF float32) []models.Playlist {
	id := uint(idF)
	if a.pl != nil && a.pl.isCurrentPlaylist(id) {
		a.Stop()
	} else {
		a.invalidatePending()
	}

	if err := inits.DB.Where("playlist_id = ?", id).Delete(&models.PlaylistItem{}).Error; err != nil {
		log.Println("RemovePlaylist: error clearing items", err)
		a.emit("audio_error", fmt.Sprintf("не удалось удалить элементы плейлиста: %s", err.Error()))
	}
	if err := inits.DB.Delete(&models.Playlist{}, id).Error; err != nil {
		log.Println("RemovePlaylist: error", err)
		a.emit("audio_error", fmt.Sprintf("не удалось удалить плейлист: %s", err.Error()))
	}

	playlists := a.ListPlaylists()
	a.emit("audio_playlists_update", playlists)
	return playlists
}

// SetPlaylistFlags правит AutoAdvance/Loop/FadeMs. Если сейчас играет элемент
// именно этого плейлиста, ранее подготовленный (или не подготовленный)
// pending мог стать неверным при смене флагов "на лету" — пересчитываем его
// сразу, а не ждём естественного следующего done.
func (a *AudioService) SetPlaylistFlags(idF float32, input PlaylistFlagsInput) *models.Playlist {
	id := uint(idF)
	updates := map[string]any{}
	if input.AutoAdvance != nil {
		updates["auto_advance"] = *input.AutoAdvance
	}
	if input.Loop != nil {
		updates["loop"] = *input.Loop
	}
	if input.FadeMs != nil {
		updates["fade_ms"] = *input.FadeMs
	}
	if len(updates) > 0 {
		if err := inits.DB.Model(&models.Playlist{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			log.Println("SetPlaylistFlags: error", err)
			a.emit("audio_error", fmt.Sprintf("не удалось сохранить настройки плейлиста: %s", err.Error()))
		}
	}

	if a.pl != nil {
		if playlistId, itemId, ok := a.pl.currentItem(); ok && playlistId == id {
			a.invalidatePending()
			a.preparePendingNext(id, itemId)
		}
	}

	playlist := a.playlistWithItems(id)
	a.emit("audio_playlists_update", a.ListPlaylists())
	return playlist
}

// AddToPlaylist добавляет трек в конец плейлиста (Position = n+1).
//
// ⚠️ Известное упрощение: если сейчас играет последний элемент плейлиста с
// AutoAdvance+Loop, уже подготовленный pending был вычислен как "обёртка на
// первый" — добавление нового элемента в конец его не отменяет, новый трек
// станет действительно последним только со следующего прохода. План не
// требует пересчёта pending на AddToPlaylist (в отличие от Remove/Reorder,
// где иначе автопереход искал бы несуществующий элемент), поэтому сознательно
// не усложняем.
func (a *AudioService) AddToPlaylist(playlistIdF, trackIdF float32) *models.Playlist {
	playlistId := uint(playlistIdF)
	trackId := uint(trackIdF)

	var count int64
	if err := inits.DB.Model(&models.PlaylistItem{}).Where("playlist_id = ?", playlistId).Count(&count).Error; err != nil {
		log.Println("AddToPlaylist: error counting items", err)
		a.emit("audio_error", fmt.Sprintf("не удалось добавить трек в плейлист: %s", err.Error()))
		return nil
	}
	item := models.PlaylistItem{PlaylistId: playlistId, TrackId: trackId, Position: int(count) + 1}
	if err := inits.DB.Create(&item).Error; err != nil {
		log.Println("AddToPlaylist: error", err)
		a.emit("audio_error", fmt.Sprintf("не удалось добавить трек в плейлист: %s", err.Error()))
		return nil
	}

	playlist := a.playlistWithItems(playlistId)
	a.emit("audio_playlists_update", a.ListPlaylists())
	return playlist
}

// RemoveFromPlaylist удаляет один элемент и перенумеровывает оставшиеся в
// 1..n. Единое правило (план, этап 4): если удаляемый элемент сейчас играет
// (или на паузе) — сначала остановить, иначе PlayerState.PlaylistId/ItemId
// после удаления указывали бы в пустоту.
func (a *AudioService) RemoveFromPlaylist(itemIdF float32) *models.Playlist {
	itemId := uint(itemIdF)
	var item models.PlaylistItem
	if err := inits.DB.First(&item, itemId).Error; err != nil {
		log.Println("RemoveFromPlaylist: item not found", err)
		return nil
	}
	playlistId := item.PlaylistId

	if a.pl != nil && a.pl.isCurrentItem(itemId) {
		a.Stop()
	} else {
		a.invalidatePending()
	}

	err := inits.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&models.PlaylistItem{}, itemId).Error; err != nil {
			return err
		}
		var remaining []models.PlaylistItem
		if err := tx.Where("playlist_id = ?", playlistId).Order("position ASC").Find(&remaining).Error; err != nil {
			return err
		}
		for i, it := range remaining {
			if it.Position == i+1 {
				continue
			}
			if err := tx.Model(&models.PlaylistItem{}).Where("id = ?", it.ID).Update("position", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Println("RemoveFromPlaylist: error", err)
		a.emit("audio_error", fmt.Sprintf("не удалось убрать трек из плейлиста: %s", err.Error()))
		return nil
	}

	playlist := a.playlistWithItems(playlistId)
	a.emit("audio_playlists_update", a.ListPlaylists())
	return playlist
}

// ReorderPlaylist проставляет позиции РОВНО по порядку переданного массива
// (1..len(itemIds)) — НЕ парными свопами, как соседние куплеты
// (db_songs.go/CreateCouplet): перетаскивание в плейлисте двигает элемент
// через весь список за одно действие, а не переставляет с соседом, и
// выражать это как последовательность обменов было бы и сложнее, и менее
// прямым способом получить тот же результат. Осознанное отличие от образца.
//
// Единое правило (план, этап 4): если сейчас играет элемент именно этого
// плейлиста — останавливаем, потому что смена позиций делает недействительным
// уже вычисленный (или готовящийся) "следующий" элемент автоперехода.
func (a *AudioService) ReorderPlaylist(playlistIdF float32, itemIds []uint) *models.Playlist {
	playlistId := uint(playlistIdF)

	if a.pl != nil && a.pl.isCurrentPlaylist(playlistId) {
		a.Stop()
	} else {
		a.invalidatePending()
	}

	err := inits.DB.Transaction(func(tx *gorm.DB) error {
		for i, id := range itemIds {
			if err := tx.Model(&models.PlaylistItem{}).
				Where("id = ? AND playlist_id = ?", id, playlistId).
				Update("position", i+1).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Println("ReorderPlaylist: error", err)
		a.emit("audio_error", fmt.Sprintf("не удалось изменить порядок плейлиста: %s", err.Error()))
		return nil
	}

	playlist := a.playlistWithItems(playlistId)
	a.emit("audio_playlists_update", a.ListPlaylists())
	return playlist
}

// step — общая реализация Next/Prev: находит текущий элемент в его плейлисте
// и запускает соседний по позиции, с оборачиванием по Loop. Работает
// независимо от AutoAdvance — это ручная навигация оператора, а не
// автоматика.
func (a *AudioService) step(dir int) error {
	if a.pl == nil {
		return fmt.Errorf("плеер недоступен")
	}
	_, itemId, ok := a.pl.currentItem()
	if !ok {
		return fmt.Errorf("ничего не играет")
	}

	var item models.PlaylistItem
	if err := inits.DB.First(&item, itemId).Error; err != nil {
		return fmt.Errorf("текущий элемент не найден: %w", err)
	}
	var playlist models.Playlist
	if err := inits.DB.First(&playlist, item.PlaylistId).Error; err != nil {
		return fmt.Errorf("плейлист не найден: %w", err)
	}
	var items []models.PlaylistItem
	if err := inits.DB.Where("playlist_id = ?", item.PlaylistId).Order("position ASC").Find(&items).Error; err != nil || len(items) == 0 {
		return fmt.Errorf("не удалось прочитать плейлист")
	}

	idx := -1
	for i, it := range items {
		if it.ID == itemId {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("текущий элемент больше не в плейлисте")
	}

	target := idx + dir
	switch {
	case target < 0:
		if !playlist.Loop {
			return fmt.Errorf("это первый элемент плейлиста")
		}
		target = len(items) - 1
	case target >= len(items):
		if !playlist.Loop {
			return fmt.Errorf("это последний элемент плейлиста")
		}
		target = 0
	}

	return a.Play(float32(item.PlaylistId), float32(items[target].ID))
}

func (a *AudioService) Next() error {
	return a.step(1)
}

func (a *AudioService) Prev() error {
	return a.step(-1)
}
