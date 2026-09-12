package main

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"changeme/backend/inits"
	"changeme/backend/models"
)

// waitForState опрашивает a.State() до выполнения cond или таймаута — движок
// в этих тестах гоняется тест-пуллером (onSamples в фоновой горутине, как
// TestConcurrentSeekDoesNotRaceDecoder), а не настоящим malgo-колбэком, так
// что переходы между треками не синхронны с вызовом Play/onSamples и их надо
// дожидаться, а не проверять сразу.
func waitForState(t *testing.T, a *AudioService, cond func(PlayerState) bool) PlayerState {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var last PlayerState
	for time.Now().Before(deadline) {
		last = a.State()
		if cond(last) {
			return last
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for expected state, last=%+v", last)
	return last
}

// driveUntil продвигает воспроизведение маленькими порциями кадров,
// проверяя состояние ПЕРЕД каждой порцией, пока не выполнится cond.
//
// ⚠️ Так, а не свободным фоновым пулом (pumpSamples) + отдельным опросом:
// без -race (и вообще на быстрой машине) вся цепочка 0 -> 1 -> 2 -> стоп на
// тестовых файлах в десятки-сотни сэмплов успевает провернуться между двумя
// соседними тиками опроса State(), и промежуточные ItemId никогда не
// замечаются — ровно так однажды и упал этот тест под `go test` (без
// -race, где всё быстрее). Опрос ПЕРЕД каждой маленькой порцией гарантирует,
// что переход не будет пропущен: пока играющий элемент не осушен полностью,
// State() между вызовами не может "перепрыгнуть" через промежуточный ItemId.
func driveUntil(t *testing.T, a *AudioService, cond func(PlayerState) bool) PlayerState {
	t.Helper()
	buf := make([]byte, 64*2*4)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st := a.State(); cond(st) {
			return st
		}
		a.pl.onSamples(buf, nil, 64)
	}
	last := a.State()
	t.Fatalf("timeout waiting for expected state, last=%+v", last)
	return last
}

// pumpSamples крутит onSamples в фоне, как это делал бы data-колбэк malgo —
// общий приём с TestConcurrentSeekDoesNotRaceDecoder (И6: движок тестируется
// без звуковой карты через тест-пуллер). Возвращает функцию остановки.
func pumpSamples(a *AudioService) (stop func()) {
	stopCh := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 512*2*4)
		for {
			select {
			case <-stopCh:
				return
			default:
			}
			a.pl.onSamples(buf, nil, 512)
		}
	}()
	return func() {
		close(stopCh)
		wg.Wait()
	}
}

// TestAutoAdvanceSequence — план: "стаб-источник, прогон плейлиста с
// AutoAdvance/Loop, порядок и остановка в конце". Вместо настоящего стаб-
// источника используются короткие тестовые wav (writeTestWav) — decodeExt
// требует реальный (пусть и тривиальный) файл, а движок ниже уже проверен на
// синтетических источниках в audio_player_test.go.
func TestAutoAdvanceSequence(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = fakeOpenDevice(8000)

	var trackIds []uint
	for i := 0; i < 3; i++ {
		src := filepath.Join(t.TempDir(), "seq.wav")
		writeTestWav(t, src, uniqueTestDuration()) // короткий трек — быстро дренируется тест-пуллером, уникальный хеш (см. uniqueTestDuration)
		res := a.ImportTrack(context.Background(), src)
		if res.Error != "" {
			t.Fatalf("ImportTrack: %s", res.Error)
		}
		trackIds = append(trackIds, res.Track.ID)
	}

	playlist := models.Playlist{Name: "Авто", AutoAdvance: true, Loop: false}
	if err := inits.DB.Create(&playlist).Error; err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	var itemIds []uint
	for i, tid := range trackIds {
		item := models.PlaylistItem{PlaylistId: playlist.ID, TrackId: tid, Position: i + 1}
		if err := inits.DB.Create(&item).Error; err != nil {
			t.Fatalf("create playlist item: %v", err)
		}
		itemIds = append(itemIds, item.ID)
	}

	if err := a.Play(float32(playlist.ID), float32(itemIds[0])); err != nil {
		t.Fatalf("Play: %v", err)
	}

	// Порядок: 0 -> 1 -> 2 -> стоп (без Loop).
	driveUntil(t, a, func(st PlayerState) bool { return st.ItemId == itemIds[1] })
	driveUntil(t, a, func(st PlayerState) bool { return st.ItemId == itemIds[2] })
	driveUntil(t, a, func(st PlayerState) bool { return st.Status == string(statusIdle) })

	// Теперь Loop: включаем и запускаем с последнего элемента — обязан
	// обернуться на первый, а не остановиться.
	trueVal := true
	a.SetPlaylistFlags(float32(playlist.ID), PlaylistFlagsInput{Loop: &trueVal})
	if err := a.Play(float32(playlist.ID), float32(itemIds[2])); err != nil {
		t.Fatalf("Play (loop phase): %v", err)
	}

	driveUntil(t, a, func(st PlayerState) bool { return st.ItemId == itemIds[0] })
}

// TestConcurrentStopDuringAutoAdvanceDoesNotRace — этап 4 добавил новый
// конкурентный путь (watchPlayerEvents -> tryAdvance -> startTrack,
// работающий с a.pending под pendingMu, конкурентно с обычными Play/Stop под
// p.mu). Гоняет реальный автопереход (Loop, пуллер вместо звуковой карты) в
// одной горутине и оператора, долбящего Stop(), в другой — ровно то самое
// "-race реально гоняет две горутины", а не только последовательный вызов
// методов. Правильность здесь — это finishIfCurrent, резервирующий gen для
// возможного автоперехода В ТОЙ ЖЕ критической секции, что и приёмку "трек
// доиграл" (см. комментарий у finishIfCurrent в audio_player.go): без этого
// Stop() посреди перехода на следующий трек имел шанс быть молча отменённым
// более новым (но на самом деле устаревшим по намерению) gen автоперехода.
func TestConcurrentStopDuringAutoAdvanceDoesNotRace(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = fakeOpenDevice(8000)

	var trackIds []uint
	for i := 0; i < 2; i++ {
		src := filepath.Join(t.TempDir(), "race.wav")
		writeTestWav(t, src, uniqueTestDuration())
		res := a.ImportTrack(context.Background(), src)
		if res.Error != "" {
			t.Fatalf("ImportTrack: %s", res.Error)
		}
		trackIds = append(trackIds, res.Track.ID)
	}

	playlist := models.Playlist{Name: "Гонка", AutoAdvance: true, Loop: true}
	if err := inits.DB.Create(&playlist).Error; err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	var itemIds []uint
	for i, tid := range trackIds {
		item := models.PlaylistItem{PlaylistId: playlist.ID, TrackId: tid, Position: i + 1}
		if err := inits.DB.Create(&item).Error; err != nil {
			t.Fatalf("create playlist item: %v", err)
		}
		itemIds = append(itemIds, item.ID)
	}

	if err := a.Play(float32(playlist.ID), float32(itemIds[0])); err != nil {
		t.Fatalf("Play: %v", err)
	}

	stopPump := pumpSamples(a)

	// Вторая горутина: оператор долбит Stop(), пока автопереход (Loop=true,
	// так что он крутится бесконечно) гоняет треки по кругу в watchPlayerEvents.
	stopClicks := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stopClicks:
				return
			default:
			}
			a.Stop()
		}
	}()

	time.Sleep(200 * time.Millisecond) // несколько кругов автоперехода конкурентно со Stop()
	close(stopClicks)
	wg.Wait()
	stopPump()

	// Финальный детерминированный Stop() после того, как обе горутины
	// остановлены — состояние обязано быть idle, что бы ни творилось выше.
	a.Stop()
	if st := a.State(); st.Status != string(statusIdle) {
		t.Fatalf("final status = %q, want idle after explicit Stop()", st.Status)
	}
}

// TestFadeOutStopStopsPlayback — сквозной тест (не только на голом chain, как
// TestFadeOutStopDrains, а через реальный AudioService.FadeOutStop()):
// плейлист с FadeMs, Play, FadeOutStop — обязано дойти до idle за конечное
// время. Без внешнего beep.Take вокруг effects.Transition (см. buildChain)
// этот тест зависал бы в driveUntil навсегда: audio_stopped/done никогда бы
// не пришли, потому что Transition сам по себе никогда не отдаёт ok=false.
func TestFadeOutStopStopsPlayback(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = fakeOpenDevice(8000)

	src := filepath.Join(t.TempDir(), "fade.wav")
	writeTestWav(t, src, 1.0) // заведомо длиннее FadeMs ниже
	res := a.ImportTrack(context.Background(), src)
	if res.Error != "" {
		t.Fatalf("ImportTrack: %s", res.Error)
	}

	playlist := models.Playlist{Name: "Фейд", FadeMs: 50}
	if err := inits.DB.Create(&playlist).Error; err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	item := models.PlaylistItem{PlaylistId: playlist.ID, TrackId: res.Track.ID, Position: 1}
	if err := inits.DB.Create(&item).Error; err != nil {
		t.Fatalf("create playlist item: %v", err)
	}

	if err := a.Play(float32(playlist.ID), float32(item.ID)); err != nil {
		t.Fatalf("Play: %v", err)
	}
	driveUntil(t, a, func(st PlayerState) bool { return st.Status == string(statusPlaying) })

	a.FadeOutStop()

	driveUntil(t, a, func(st PlayerState) bool { return st.Status == string(statusIdle) })
}
