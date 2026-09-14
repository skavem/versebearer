package main

import (
	"fmt"
	"log"

	"changeme/backend/inits"
	"changeme/backend/models"

	"gorm.io/gorm"
)

// Плейлисты: CRUD, порядок элементов и ручная навигация по списку
// (Next/Prev). Удаление самих треков из медиатеки — правило «файл живёт,
// пока на него есть хоть одна ссылка» — живёт в audio_library.go
// (deleteTrackIfUnused): отсюда его зовут RemoveFromPlaylist/ClearPlaylist/
// RemovePlaylist, но это работа медиатеки, а не плейлиста.

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

// RemovePlaylist удаляет плейлист и все его элементы. Если сейчас играет
// (или на паузе) что-то из ИМЕННО этого плейлиста — воспроизведение
// останавливается первым делом, иначе горутина автоперехода станет искать
// следующий элемент среди мягко удалённых строк. Если играет что-то из
// ДРУГОГО плейлиста — не трогаем ни его, ни его pending: мутация чужого
// плейлиста не физическая причина остановки (см. модель состояния,
// AGENTS.md).
//
// ⚠️ HIGH №2 обзора: раньше удалялись только строки PlaylistItem и сам
// плейлист, а треки в медиатеке — никогда. Самый естественный жест уборки
// (экрана медиатеки больше нет — оператор удаляет плейлист целиком после
// служения) копил файлы в %LOCALAPPDATA% недостижимыми из UI навсегда.
// По сути RemovePlaylist = ClearPlaylist + удаление строки плейлиста:
// список удаляемых элементов читается ДО удаления (как в ClearPlaylist),
// а после — тот же дедуплицированный по TrackId проход через
// deleteTrackIfUnused (один трек может быть в нескольких плейлистах —
// удаляем файл только когда исчезла последняя ссылка).
//
// LOW обзора: последний плейлист не удаляется — версия БД уже "8",
// seedDefaultPlaylist больше не сработает при следующем старте, и
// импортировать станет некуда (тот же приём, что RemoveTranslation
// применяет к последнему переводу).
func (a *AudioService) RemovePlaylist(idF float32) []models.Playlist {
	id := uint(idF)

	var count int64
	if err := inits.DB.Model(&models.Playlist{}).Count(&count).Error; err != nil {
		log.Println("RemovePlaylist: error counting playlists", err)
		a.emit("audio_error", fmt.Sprintf("не удалось проверить количество плейлистов: %s", err.Error()))
		return a.ListPlaylists()
	}
	if count <= 1 {
		a.emit("audio_error", "нельзя удалить последний плейлист — импортировать станет некуда")
		return a.ListPlaylists()
	}

	if a.pl != nil && a.pl.isCurrentPlaylist(id) {
		a.Stop()
	}

	var items []models.PlaylistItem
	if err := inits.DB.Where("playlist_id = ?", id).Find(&items).Error; err != nil {
		log.Println("RemovePlaylist: error reading items", err)
		a.emit("audio_error", fmt.Sprintf("не удалось прочитать элементы плейлиста: %s", err.Error()))
	}

	if err := inits.DB.Where("playlist_id = ?", id).Delete(&models.PlaylistItem{}).Error; err != nil {
		log.Println("RemovePlaylist: error clearing items", err)
		a.emit("audio_error", fmt.Sprintf("не удалось удалить элементы плейлиста: %s", err.Error()))
	}
	if err := inits.DB.Delete(&models.Playlist{}, id).Error; err != nil {
		log.Println("RemovePlaylist: error", err)
		a.emit("audio_error", fmt.Sprintf("не удалось удалить плейлист: %s", err.Error()))
	}

	a.deleteUnusedTracksFor(items)

	playlists := a.ListPlaylists()
	a.emit("audio_playlists_update", playlists)
	return playlists
}

// SetPlaylistFlags правит AutoAdvance/Loop/FadeMs. Если сейчас играет элемент
// именно этого плейлиста, ранее подготовленный (или не подготовленный)
// pending мог стать неверным при смене флагов "на лету" — refreshPending
// пересчитывает его сразу (и не переоткрывает декодер, если ответ на самом
// деле не изменился — см. preparePendingNext), а не ждёт естественного
// следующего done.
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

	a.refreshPendingForPlaylist(id)

	playlist := a.playlistWithItems(id)
	a.emit("audio_playlists_update", a.ListPlaylists())
	return playlist
}

// AddToPlaylist добавляет трек в конец плейлиста (Position = n+1). Если
// сейчас играет элемент именно этого плейлиста, пересчитываем pending —
// добавленный трек мог стать новым "следующим" (например, играет последний
// элемент плейлиста с AutoAdvance+Loop: раньше pending был вычислен как
// "обёртка на первый", а с новым элементом в конце правильный ответ —
// только что добавленный трек).
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

	a.refreshPendingForPlaylist(playlistId)

	playlist := a.playlistWithItems(playlistId)
	a.emit("audio_playlists_update", a.ListPlaylists())
	return playlist
}

// AddTracksToPlaylist — как AddToPlaylist, но пачкой за один вызов: одна
// транзакция, один audio_playlists_update. Нужен импорту (audioStore.
// importFilesToPlaylist): раньше он звал AddToPlaylist по одному на файл, и
// КАЖДЫЙ вызов эмитил полный список плейлистов — на 50 файлах 50 лишних
// перерисовок фронта во время работающего звука. trackIds — уже успешно
// импортированные (порядок сохраняется, как позиция в конце списка).
func (a *AudioService) AddTracksToPlaylist(playlistIdF float32, trackIds []uint) *models.Playlist {
	playlistId := uint(playlistIdF)
	if len(trackIds) == 0 {
		return a.playlistWithItems(playlistId)
	}

	// Count — ВНУТРИ транзакции, а не до неё (MEDIUM №4 обзора): оператор,
	// бросивший в плейлист вторую пачку файлов секундой позже первой, иначе
	// получил бы обе Count() ДО того, как первая пачка успеет вставить свои
	// строки — обе транзакции насчитали бы одинаковый count и получили
	// пересекающиеся Position, порядок списка стал бы произволен. Фронт
	// (audioStore.importFiles) дополнительно не даёт войти второй раз, пока
	// первый импорт не завершился — это вторая, независимая линия защиты.
	err := inits.DB.Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&models.PlaylistItem{}).Where("playlist_id = ?", playlistId).Count(&count).Error; err != nil {
			return err
		}
		for i, trackId := range trackIds {
			item := models.PlaylistItem{PlaylistId: playlistId, TrackId: trackId, Position: int(count) + i + 1}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Println("AddTracksToPlaylist: error", err)
		a.emit("audio_error", fmt.Sprintf("не удалось добавить треки в плейлист: %s", err.Error()))
		return nil
	}

	a.refreshPendingForPlaylist(playlistId)

	playlist := a.playlistWithItems(playlistId)
	a.emit("audio_playlists_update", a.ListPlaylists())
	return playlist
}

// RemoveFromPlaylist удаляет один элемент и перенумеровывает оставшиеся в
// 1..n. Позиция и DB-строка элемента — не звук: удаление НЕ останавливает
// воспроизведение само по себе, даже если удаляемый элемент сейчас играет —
// открытый декодер уже не зависит от этой строки. Единственная настоящая
// причина стопа — sharing violation при удалении ФАЙЛА, когда трек больше
// нигде не используется, и это решение целиком живёт в deleteTrackIfUnused
// (isCurrentTrack там), а не здесь.
//
// Правка 1 («убрать медиатеку как отдельный экран»): без отдельного экрана
// осиротевшая запись AudioTrack стала бы недостижимой из UI, а её файл
// копился бы в %LOCALAPPDATA% незаметно и навсегда. Поэтому после удаления
// элемента трек проверяется на ссылки (deleteTrackIfUnused) — дедупликация
// по хешу означает, что один трек может быть в нескольких плейлистах, и
// удалять файл можно только когда исчезла последняя ссылка.
func (a *AudioService) RemoveFromPlaylist(itemIdF float32) *models.Playlist {
	itemId := uint(itemIdF)
	var item models.PlaylistItem
	if err := inits.DB.First(&item, itemId).Error; err != nil {
		log.Println("RemoveFromPlaylist: item not found", err)
		return nil
	}
	playlistId := item.PlaylistId
	trackId := item.TrackId
	removedPosition := item.Position // см. refreshPendingAfterRemoval ниже — снимается ДО удаления

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

	a.deleteTrackIfUnused(trackId)

	// Удалённый элемент мог быть тем, на кого указывал pending играющего
	// именно этого плейлиста (deleteTrackIfUnused решает вопрос стопа только
	// по trackId, не по позиции в списке) — пересчитываем ответ на "что
	// дальше" заново; если он не изменился, decodePendingNext не перезапустится
	// (см. preparePendingNext).
	//
	// ⚠️ HIGH №1 обзора: если удалённый элемент — САМ играющий (currentItem
	// == itemId), обычный refreshPendingForPlaylist бесполезен —
	// nextPlaylistItem ищет afterItemId в уже актуальном списке, а строка
	// только что удалена (idx=-1), и preparePendingNext молча сдаётся: трек
	// доигрывает, а автопереход считает, что дальше ничего нет, хотя в
	// списке остались треки. refreshPendingAfterRemoval знает removedPosition
	// и находит честного "следующего" в обход этой дыры.
	if a.pl != nil {
		if pid, curItemId, ok := a.pl.currentItem(); ok && pid == playlistId && curItemId == itemId {
			a.refreshPendingAfterRemoval(playlistId, itemId, removedPosition)
		} else {
			a.refreshPendingForPlaylist(playlistId)
		}
	}

	playlist := a.playlistWithItems(playlistId)
	a.emit("audio_playlists_update", a.ListPlaylists())
	return playlist
}

// ClearPlaylist удаляет ВСЕ элементы плейлиста разом — кнопка «Удалить всё»
// (правка 1). Правило со ссылками единое с RemoveFromPlaylist: каждый
// затронутый трек проверяется на count>0 в ОСТАВШИХСЯ (не только этого
// плейлиста) PlaylistItem, удаляется только если ссылок больше нет. Если
// сейчас играет элемент ДРУГОГО плейлиста — не трогаем ни его, ни его
// pending.
func (a *AudioService) ClearPlaylist(idF float32) *models.Playlist {
	id := uint(idF)

	if a.pl != nil && a.pl.isCurrentPlaylist(id) {
		// LOW обзора, пункт 6, осознанное исключение: стоп здесь идёт по
		// факту "это играющий плейлист", ДО того, как выяснится, будет ли
		// вообще удалён хоть один файл (треки, разделённые с другим
		// плейлистом, на диске останутся). Формально расходится с моделью
		// "стоп только по физической причине" (см. deleteTrackIfUnused), но
		// «Удалить всё» по играющему плейлисту и так должно останавливать
		// звук — это ожидаемое поведение самой кнопки, не побочный эффект
		// физики файлов. Не переделывать.
		a.Stop()
	}

	var items []models.PlaylistItem
	if err := inits.DB.Where("playlist_id = ?", id).Find(&items).Error; err != nil {
		log.Println("ClearPlaylist: error reading items", err)
		a.emit("audio_error", fmt.Sprintf("не удалось прочитать плейлист: %s", err.Error()))
		return a.playlistWithItems(id)
	}

	if err := inits.DB.Where("playlist_id = ?", id).Delete(&models.PlaylistItem{}).Error; err != nil {
		log.Println("ClearPlaylist: error", err)
		a.emit("audio_error", fmt.Sprintf("не удалось очистить плейлист: %s", err.Error()))
		return a.playlistWithItems(id)
	}

	a.deleteUnusedTracksFor(items)

	playlist := a.playlistWithItems(id)
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
// ⚠️ Перестановка НЕ трогает звук. Position — свойство элемента, не его
// identity: воспроизведение привязано к itemId и уже открытому декодеру, а
// не к месту в списке (см. модель состояния, AGENTS.md), поэтому смена
// порядка не имеет права останавливать то, что играет — это была
// первоначальная жалоба оператора ("переставил элементы — звук оборвался").
// После транзакции — один refreshPendingForPlaylist, и только если сейчас
// играет элемент ИМЕННО этого плейлиста: перестановка могла изменить ответ
// на "что дальше после текущего", и его нужно пересчитать, но не переоткрыть
// декодер зря, если ответ на самом деле не изменился (см. preparePendingNext).
func (a *AudioService) ReorderPlaylist(playlistIdF float32, itemIds []uint) *models.Playlist {
	playlistId := uint(playlistIdF)

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

	a.refreshPendingForPlaylist(playlistId)

	playlist := a.playlistWithItems(playlistId)
	a.emit("audio_playlists_update", a.ListPlaylists())
	return playlist
}

// itemIndex — позиция элемента itemId в уже упорядоченном (Position ASC)
// списке, или -1, если его там нет. Одинаково нужна и автопереходу
// (nextPlaylistItem), и ручной навигации (step ниже); оба одинаково трактуют
// -1 как «элемент удалён или переставлен в другой плейлист между вызовами» и
// не пытаются угадывать позицию по устаревшим данным.
func itemIndex(items []models.PlaylistItem, itemId uint) int {
	for i, it := range items {
		if it.ID == itemId {
			return i
		}
	}
	return -1
}

// step — общая реализация Next/Prev: находит текущий элемент в его плейлисте
// и запускает соседний по позиции, с оборачиванием по Loop. Работает
// независимо от AutoAdvance — это ручная навигация оператора, а не
// автоматика.
//
// playlistId берётся у самого плеера (currentItem), а не перечитывается из
// строки элемента: элемент не переезжает между плейлистами, так что второй
// источник той же правды был бы лишним запросом с собственным путём отказа.
func (a *AudioService) step(dir int) error {
	if a.pl == nil {
		return fmt.Errorf("плеер недоступен")
	}
	playlistId, itemId, ok := a.pl.currentItem()
	if !ok {
		return fmt.Errorf("ничего не играет")
	}

	var playlist models.Playlist
	if err := inits.DB.First(&playlist, playlistId).Error; err != nil {
		return fmt.Errorf("плейлист не найден: %w", err)
	}
	var items []models.PlaylistItem
	if err := inits.DB.Where("playlist_id = ?", playlistId).Order("position ASC").Find(&items).Error; err != nil || len(items) == 0 {
		return fmt.Errorf("не удалось прочитать плейлист")
	}

	idx := itemIndex(items, itemId)
	if idx < 0 {
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

	return a.Play(float32(playlistId), float32(items[target].ID))
}

func (a *AudioService) Next() error {
	return a.step(1)
}

func (a *AudioService) Prev() error {
	return a.step(-1)
}
