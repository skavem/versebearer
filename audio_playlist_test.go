package main

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/paths"
)

// testTrackSeq — см. uniqueTestDuration.
var testTrackSeq atomic.Int64

// createTestTrack импортирует минимальный тестовый wav и возвращает его id —
// общий шаг для тестов плейлистов, которым нужен настоящий (пусть и
// тривиальный) AudioTrack, чтобы Preload("Items.Track") давало осмысленные
// данные.
//
// ⚠️ paths.MediaDir() мемоизирован (sync.Once) на весь процесс тестов
// (audio_import_test.go, TestMain) — один физический каталог на все тесты
// в бинарнике. writeTestWav с одинаковой длительностью в разных тестах даёт
// БИТ-В-БИТ одинаковое содержимое, а значит и одинаковый sha256 → дедуп по
// хешу склеил бы их в один трек, а на Windows второй rename на уже
// существующее (и не факт что закрытое — см. audio_autoadvance.go, pending
// держит декодер открытым фоново) имя падает как "Access is denied". Поэтому
// каждый вызов получает заведомо уникальную длительность.
func uniqueTestDuration() float64 {
	testTrackSeq.Add(1)
	return 0.05 + float64(testTrackSeq.Load())*0.001
}

func createTestTrack(t *testing.T, name string) uint {
	t.Helper()
	a := &AudioService{}
	src := filepath.Join(t.TempDir(), name+".wav")
	writeTestWav(t, src, uniqueTestDuration())
	res := a.ImportTrack(context.Background(), src)
	if res.Error != "" {
		t.Fatalf("ImportTrack(%s): %s", name, res.Error)
	}
	return res.Track.ID
}

// TestAddToPlaylistAssignsNextPosition — план: "AddToPlaylist даёт
// Position = n+1".
func TestAddToPlaylistAssignsNextPosition(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	playlist := a.CreatePlaylist("Тест")
	if playlist == nil {
		t.Fatal("CreatePlaylist returned nil")
	}
	t1 := createTestTrack(t, "t1")
	t2 := createTestTrack(t, "t2")
	t3 := createTestTrack(t, "t3")

	p1 := a.AddToPlaylist(float32(playlist.ID), float32(t1))
	if len(p1.Items) != 1 || p1.Items[0].Position != 1 {
		t.Fatalf("after first add: items=%+v", p1.Items)
	}

	a.AddToPlaylist(float32(playlist.ID), float32(t2))
	p3 := a.AddToPlaylist(float32(playlist.ID), float32(t3))
	if len(p3.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(p3.Items))
	}
	for i, it := range p3.Items {
		if it.Position != i+1 {
			t.Errorf("item %d: Position = %d, want %d", i, it.Position, i+1)
		}
	}
	if p3.Items[2].TrackId != t3 {
		t.Errorf("last item trackId = %d, want %d (Position должен идти n+1, не в начало)", p3.Items[2].TrackId, t3)
	}
}

// TestAddTracksToPlaylistBatchesPositions — AddTracksToPlaylist (пакетный
// импорт): один вызов добавляет N треков подряд с Position = n+1..n+N, как
// N последовательных AddToPlaylist, но за одну транзакцию/один emit.
func TestAddTracksToPlaylistBatchesPositions(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	playlist := a.CreatePlaylist("Пакет")
	t1 := createTestTrack(t, "b1")
	t2 := createTestTrack(t, "b2")
	t3 := createTestTrack(t, "b3")

	// Один уже существующий элемент — новые обязаны продолжить нумерацию, а
	// не начать её заново с 1.
	a.AddToPlaylist(float32(playlist.ID), float32(t1))

	result := a.AddTracksToPlaylist(float32(playlist.ID), []uint{t2, t3})
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result.Items))
	}
	for i, it := range result.Items {
		if it.Position != i+1 {
			t.Errorf("item %d: Position = %d, want %d", i, it.Position, i+1)
		}
	}
	if result.Items[1].TrackId != t2 || result.Items[2].TrackId != t3 {
		t.Errorf("batch order not preserved: %+v", result.Items)
	}
}

// TestRemoveFromPlaylistRenumbers — план: "RemoveFromPlaylist перенумеровывает
// 1..n".
func TestRemoveFromPlaylistRenumbers(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	playlist := a.CreatePlaylist("Тест")
	var trackIds []uint
	for i := 0; i < 4; i++ {
		trackIds = append(trackIds, createTestTrack(t, "t"))
	}
	var itemIds []uint
	for _, tid := range trackIds {
		p := a.AddToPlaylist(float32(playlist.ID), float32(tid))
		itemIds = append(itemIds, p.Items[len(p.Items)-1].ID)
	}

	// Удаляем второй по счёту (Position=2) — оставшиеся три обязаны стать
	// 1,2,3 без дырки на месте удалённого.
	result := a.RemoveFromPlaylist(float32(itemIds[1]))
	if len(result.Items) != 3 {
		t.Fatalf("expected 3 items after removal, got %d", len(result.Items))
	}
	for i, it := range result.Items {
		if it.Position != i+1 {
			t.Errorf("item %d: Position = %d, want %d", i, it.Position, i+1)
		}
		if it.ID == itemIds[1] {
			t.Error("removed item still present")
		}
	}
}

// TestReorderPlaylistExactOrder — план: "ReorderPlaylist переставляет ровно
// по переданному порядку" (не парными свопами, как куплеты).
func TestReorderPlaylistExactOrder(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	playlist := a.CreatePlaylist("Тест")
	var itemIds []uint
	for i := 0; i < 4; i++ {
		tid := createTestTrack(t, "t")
		p := a.AddToPlaylist(float32(playlist.ID), float32(tid))
		itemIds = append(itemIds, p.Items[len(p.Items)-1].ID)
	}
	// itemIds сейчас в порядке [0,1,2,3] (Position 1..4). Двигаем последний
	// элемент в начало целиком — не соседний своп, а перестановка через весь
	// список одним действием.
	newOrder := []uint{itemIds[3], itemIds[0], itemIds[1], itemIds[2]}
	result := a.ReorderPlaylist(float32(playlist.ID), newOrder)
	if len(result.Items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(result.Items))
	}
	for i, it := range result.Items {
		if it.ID != newOrder[i] {
			t.Errorf("position %d: itemId = %d, want %d", i+1, it.ID, newOrder[i])
		}
		if it.Position != i+1 {
			t.Errorf("item %d: Position = %d, want %d", it.ID, it.Position, i+1)
		}
	}
}

// TestRemoveTrackClearsPlaylistItems — план: "удаление трека вычищает его из
// всех плейлистов".
func TestRemoveTrackClearsPlaylistItems(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	p1 := a.CreatePlaylist("П1")
	p2 := a.CreatePlaylist("П2")
	trackId := createTestTrack(t, "shared")

	a.AddToPlaylist(float32(p1.ID), float32(trackId))
	a.AddToPlaylist(float32(p2.ID), float32(trackId))

	if err := a.RemoveTrack(float32(trackId)); err != nil {
		t.Fatalf("RemoveTrack: %v", err)
	}

	var count int64
	if err := inits.DB.Model(&models.PlaylistItem{}).Where("track_id = ?", trackId).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("PlaylistItem rows referencing removed track still present: %d", count)
	}
}

// TestRemovePlayingItemStopsPlayback — план: "удаление играющего элемента
// останавливает воспроизведение".
func TestRemovePlayingItemStopsPlayback(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = fakeOpenDevice(8000)

	playlist := a.CreatePlaylist("Тест")
	trackId := createTestTrack(t, "playing")
	p := a.AddToPlaylist(float32(playlist.ID), float32(trackId))
	itemId := p.Items[0].ID

	if err := a.Play(float32(playlist.ID), float32(itemId)); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if st := a.State(); st.Status != string(statusPlaying) {
		t.Fatalf("expected playing, got %q", st.Status)
	}

	a.RemoveFromPlaylist(float32(itemId))

	if st := a.State(); st.Status != string(statusIdle) {
		t.Errorf("removing the playing item should stop playback, status = %q", st.Status)
	}
}

// TestRemoveFromPlaylistDeletesUnusedTrack — правка 1: удаление элемента
// удаляет трек (и его файл) из медиатеки, если трек больше не используется
// ни в одном плейлисте.
func TestRemoveFromPlaylistDeletesUnusedTrack(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	playlist := a.CreatePlaylist("Тест")
	trackId := createTestTrack(t, "solo")
	p := a.AddToPlaylist(float32(playlist.ID), float32(trackId))
	itemId := p.Items[0].ID

	var track models.AudioTrack
	if err := inits.DB.First(&track, trackId).Error; err != nil {
		t.Fatalf("read track before removal: %v", err)
	}
	mediaDir, err := paths.MediaDir()
	if err != nil {
		t.Fatalf("MediaDir: %v", err)
	}
	filePath := filepath.Join(mediaDir, track.FileName)
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("track file should exist before removal: %v", err)
	}

	a.RemoveFromPlaylist(float32(itemId))

	if err := inits.DB.First(&models.AudioTrack{}, trackId).Error; err == nil {
		t.Error("track row should be deleted once its last playlist reference is gone")
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Errorf("track file should be deleted from disk, stat err = %v", err)
	}
}

// TestRemoveFromPlaylistKeepsSharedTrack — правка 1: НЕ удаляет трек, если
// он всё ещё используется в другом плейлисте (дедупликация по хешу — один
// трек, несколько PlaylistItem).
func TestRemoveFromPlaylistKeepsSharedTrack(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	p1 := a.CreatePlaylist("П1")
	p2 := a.CreatePlaylist("П2")
	trackId := createTestTrack(t, "shared")

	item1 := a.AddToPlaylist(float32(p1.ID), float32(trackId)).Items[0]
	a.AddToPlaylist(float32(p2.ID), float32(trackId))

	a.RemoveFromPlaylist(float32(item1.ID))

	if err := inits.DB.First(&models.AudioTrack{}, trackId).Error; err != nil {
		t.Error("track row should survive while another playlist still references it")
	}
}

// TestClearPlaylistDeletesUnusedTracks — «Удалить всё» ведёт себя как
// одиночный RemoveFromPlaylist на каждый элемент: удаляет треки, у которых
// не осталось ссылок, и сохраняет те, что ещё используются в другом
// плейлисте.
func TestClearPlaylistDeletesUnusedTracks(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	solo := a.CreatePlaylist("Соло")
	shared := a.CreatePlaylist("Общий")

	soloTrackId := createTestTrack(t, "solo-clear")
	sharedTrackId := createTestTrack(t, "shared-clear")

	a.AddToPlaylist(float32(solo.ID), float32(soloTrackId))
	a.AddToPlaylist(float32(solo.ID), float32(sharedTrackId))
	a.AddToPlaylist(float32(shared.ID), float32(sharedTrackId))

	result := a.ClearPlaylist(float32(solo.ID))
	if len(result.Items) != 0 {
		t.Fatalf("expected 0 items after ClearPlaylist, got %d", len(result.Items))
	}

	if err := inits.DB.First(&models.AudioTrack{}, soloTrackId).Error; err == nil {
		t.Error("solo track should be deleted, it had no other references")
	}
	if err := inits.DB.First(&models.AudioTrack{}, sharedTrackId).Error; err != nil {
		t.Error("shared track should survive, still referenced by the other playlist")
	}
}

// TestReorderPlayingPlaylistDoesNotStopPlayback — первоначальная жалоба
// оператора: перестановка элементов игравшего плейлиста обрывала звук.
// Position — свойство элемента, не его identity (см. модель состояния,
// AGENTS.md) — перестановка не имеет права касаться звука.
func TestReorderPlayingPlaylistDoesNotStopPlayback(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = fakeOpenDevice(8000)

	playlist := a.CreatePlaylist("Порядок")
	var itemIds []uint
	for i := 0; i < 3; i++ {
		tid := createTestTrack(t, "reorder")
		p := a.AddToPlaylist(float32(playlist.ID), float32(tid))
		itemIds = append(itemIds, p.Items[len(p.Items)-1].ID)
	}

	if err := a.Play(float32(playlist.ID), float32(itemIds[0])); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if st := a.State(); st.Status != string(statusPlaying) {
		t.Fatalf("expected playing, got %q", st.Status)
	}
	playingBeforeReorder := a.State().TrackId

	// Двигаем играющий элемент (Position=1) в конец списка целиком — та
	// самая перестановка, которая раньше звала a.Stop().
	newOrder := []uint{itemIds[1], itemIds[2], itemIds[0]}
	a.ReorderPlaylist(float32(playlist.ID), newOrder)

	st := a.State()
	if st.Status != string(statusPlaying) {
		t.Errorf("reordering the playing playlist stopped playback, status = %q, want %q", st.Status, statusPlaying)
	}
	if st.TrackId != playingBeforeReorder || st.ItemId != itemIds[0] {
		t.Errorf("reorder changed what's playing: TrackId=%d ItemId=%d, want TrackId=%d ItemId=%d", st.TrackId, st.ItemId, playingBeforeReorder, itemIds[0])
	}
}

// TestRemoveNonPlayingItemDoesNotBreakAutoAdvance — удаление НЕ играющего
// элемента (в данном случае — того, что шёл СЛЕДУЮЩИМ) не должно оставить
// автопереход без пары: refreshPending обязан пересчитать ответ и перескочить
// через удалённый элемент на следующий за ним, а не потерять pending молча.
func TestRemoveNonPlayingItemDoesNotBreakAutoAdvance(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = fakeOpenDevice(8000)

	playlist := models.Playlist{Name: "Пропуск", AutoAdvance: true}
	if err := inits.DB.Create(&playlist).Error; err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	var itemIds []uint
	for i := 0; i < 3; i++ {
		tid := createTestTrack(t, "skip")
		item := models.PlaylistItem{PlaylistId: playlist.ID, TrackId: tid, Position: i + 1}
		if err := inits.DB.Create(&item).Error; err != nil {
			t.Fatalf("create item: %v", err)
		}
		itemIds = append(itemIds, item.ID)
	}

	if err := a.Play(float32(playlist.ID), float32(itemIds[0])); err != nil {
		t.Fatalf("Play: %v", err)
	}

	// Удаляем средний элемент (itemIds[1]) — он не играет, но был "следующим"
	// для preparePendingNext. После удаления играющий трек обязан
	// перейти прямо на itemIds[2], а не остановиться молча.
	a.RemoveFromPlaylist(float32(itemIds[1]))

	driveUntil(t, a, func(st PlayerState) bool { return st.ItemId == itemIds[2] })
}

// TestRemovePlayingItemKeepsPlayingWhenTrackSharedElsewhere — единственная
// настоящая причина остановки при удалении элемента — sharing violation при
// удалении ФАЙЛА (deleteTrackIfUnused). Если трек ещё используется в другом
// плейлисте, файл не удаляется, значит и стоп не нужен: играющий трек
// доигрывает до конца.
func TestRemovePlayingItemKeepsPlayingWhenTrackSharedElsewhere(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = fakeOpenDevice(8000)

	p1 := a.CreatePlaylist("П1")
	p2 := a.CreatePlaylist("П2")
	trackId := createTestTrack(t, "shared-playing")

	item1 := a.AddToPlaylist(float32(p1.ID), float32(trackId)).Items[0]
	a.AddToPlaylist(float32(p2.ID), float32(trackId))

	if err := a.Play(float32(p1.ID), float32(item1.ID)); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if st := a.State(); st.Status != string(statusPlaying) {
		t.Fatalf("expected playing, got %q", st.Status)
	}

	a.RemoveFromPlaylist(float32(item1.ID))

	if st := a.State(); st.Status != string(statusPlaying) {
		t.Errorf("removing a playing item should not stop playback while its track is still referenced by another playlist, status = %q", st.Status)
	}
}

// TestRemovePlayingItemAdvancesWhenTrackSharedElsewhere — HIGH №1 обзора:
// удаление ИГРАЮЩЕГО элемента, чей трек ещё используется в другом плейлисте
// (значит физической причины для стопа нет — см.
// TestRemovePlayingItemKeepsPlayingWhenTrackSharedElsewhere), в плейлисте с
// AutoAdvance обрывало автопереход тишиной: afterItemId/finished.itemId
// удалённого элемента больше не находится в актуальном списке (idx=-1,
// nextPlaylistItem), и ни честный refreshPending (RemoveFromPlaylist), ни
// синхронный buildPendingSync-предохранитель (tryAdvance) этого не
// переживали — тот же дефект, ради которого писался весь рефакторинг
// pending, просто с другого входа. Обязан падать (таймаутом driveUntil) на
// коде до правки: играющий трек доигрывает и останавливается вместо
// перехода на itemIds[2].
func TestRemovePlayingItemAdvancesWhenTrackSharedElsewhere(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = fakeOpenDevice(8000)

	other := a.CreatePlaylist("Фон")

	playlist := models.Playlist{Name: "Основной", AutoAdvance: true}
	if err := inits.DB.Create(&playlist).Error; err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	var itemIds, trackIds []uint
	for i := 0; i < 3; i++ {
		tid := createTestTrack(t, "shared-advance")
		trackIds = append(trackIds, tid)
		item := models.PlaylistItem{PlaylistId: playlist.ID, TrackId: tid, Position: i + 1}
		if err := inits.DB.Create(&item).Error; err != nil {
			t.Fatalf("create item: %v", err)
		}
		itemIds = append(itemIds, item.ID)
	}
	// Трек второго элемента (Position=2) — тот, что будет играть и будет
	// удалён — дублируем в другой плейлист, чтобы deleteTrackIfUnused не
	// нашёл его неиспользуемым и не остановил воспроизведение сам (тогда
	// тест проверял бы TestRemovePlayingItemStopsPlayback, а не автопереход).
	a.AddToPlaylist(float32(other.ID), float32(trackIds[1]))

	if err := a.Play(float32(playlist.ID), float32(itemIds[1])); err != nil {
		t.Fatalf("Play: %v", err)
	}
	if st := a.State(); st.Status != string(statusPlaying) {
		t.Fatalf("expected playing, got %q", st.Status)
	}

	a.RemoveFromPlaylist(float32(itemIds[1]))

	// Играющий (уже удалённый из БД) элемент доигрывает, затем обязан
	// перейти на itemIds[2], а не остановиться молча тишиной.
	driveUntil(t, a, func(st PlayerState) bool { return st.ItemId == itemIds[2] })
}

// TestRemovePlaylistDeletesUnusedTracks — HIGH №2 обзора: RemovePlaylist
// удалял только строки PlaylistItem и сам плейлист, но никогда не звал
// deleteTrackIfUnused — самый естественный жест уборки (экрана медиатеки
// нет, оператор удаляет плейлист целиком после служения) копил файлы в
// медиатеке недостижимыми из UI навсегда. По сути RemovePlaylist теперь =
// ClearPlaylist + удаление строки плейлиста: то же правило дедупликации по
// ссылкам, что и TestClearPlaylistDeletesUnusedTracks.
func TestRemovePlaylistDeletesUnusedTracks(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	solo := a.CreatePlaylist("Соло")
	shared := a.CreatePlaylist("Общий")

	soloTrackId := createTestTrack(t, "solo-removeplaylist")
	sharedTrackId := createTestTrack(t, "shared-removeplaylist")

	a.AddToPlaylist(float32(solo.ID), float32(soloTrackId))
	a.AddToPlaylist(float32(solo.ID), float32(sharedTrackId))
	a.AddToPlaylist(float32(shared.ID), float32(sharedTrackId))

	a.RemovePlaylist(float32(solo.ID))

	if err := inits.DB.First(&models.AudioTrack{}, soloTrackId).Error; err == nil {
		t.Error("solo track should be deleted, it had no other references after RemovePlaylist")
	}
	if err := inits.DB.First(&models.AudioTrack{}, sharedTrackId).Error; err != nil {
		t.Error("shared track should survive, still referenced by the other playlist")
	}
}

// TestRemovePlaylistRefusesLastOne — LOW обзора: как RemoveTranslation для
// переводов (db_import.go) — RemovePlaylist не даёт остаться совсем без
// плейлиста: миграция версии 8 сеет ровно один
// (backend/inits/db.go/seedDefaultPlaylist), и без него импортировать
// станет некуда, а повторный запуск seedDefaultPlaylist уже не сработает
// (версия БД уже "8").
func TestRemovePlaylistRefusesLastOne(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	only := a.CreatePlaylist("Единственный")

	result := a.RemovePlaylist(float32(only.ID))
	if len(result) != 1 || result[0].ID != only.ID {
		t.Fatalf("last playlist must survive RemovePlaylist, got %+v", result)
	}
}

// TestNextPrevNavigation проверяет ручную навигацию (Next/Prev) — она
// работает независимо от AutoAdvance (это действие оператора, не автоматика)
// и оборачивается по Loop так же, как автопереход.
func TestNextPrevNavigation(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = fakeOpenDevice(8000)

	playlist := models.Playlist{Name: "Навигация", Loop: true}
	if err := inits.DB.Create(&playlist).Error; err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	var itemIds []uint
	for i := 0; i < 3; i++ {
		tid := createTestTrack(t, "nav")
		item := models.PlaylistItem{PlaylistId: playlist.ID, TrackId: tid, Position: i + 1}
		if err := inits.DB.Create(&item).Error; err != nil {
			t.Fatalf("create item: %v", err)
		}
		itemIds = append(itemIds, item.ID)
	}

	if err := a.Play(float32(playlist.ID), float32(itemIds[0])); err != nil {
		t.Fatalf("Play: %v", err)
	}

	if err := a.Next(); err != nil {
		t.Fatalf("Next: %v", err)
	}
	if st := a.State(); st.ItemId != itemIds[1] {
		t.Fatalf("after Next: ItemId = %d, want %d", st.ItemId, itemIds[1])
	}

	if err := a.Prev(); err != nil {
		t.Fatalf("Prev: %v", err)
	}
	if st := a.State(); st.ItemId != itemIds[0] {
		t.Fatalf("after Prev: ItemId = %d, want %d", st.ItemId, itemIds[0])
	}

	// Prev с первого элемента при Loop=true оборачивается на последний.
	if err := a.Prev(); err != nil {
		t.Fatalf("Prev (wrap): %v", err)
	}
	if st := a.State(); st.ItemId != itemIds[2] {
		t.Fatalf("after wrap Prev: ItemId = %d, want %d", st.ItemId, itemIds[2])
	}
}
