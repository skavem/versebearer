package main

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"changeme/backend/inits"
	"changeme/backend/models"

	"github.com/gen2brain/malgo"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/generators"
)

// fakeOpenDevice симулирует ensureDevice() без единого вызова malgo — И6 в
// применении к самому устройству. rate — частота, которую в бою вернул бы
// dev.SampleRate(). ctx/dev остаются nil и ни разу не разыменовываются:
// все обращения к ним в audio_device.go идут под "if dev/ctx != nil".
func fakeOpenDevice(rate uint32) func(string) (*malgo.AllocatedContext, *malgo.Device, uint32, string, error) {
	return func(string) (*malgo.AllocatedContext, *malgo.Device, uint32, string, error) {
		return nil, nil, rate, "Тестовое устройство", nil
	}
}

// setupPlayerTestDB — как setupAudioTestDB (audio_import_test.go), плюс
// GlobalState: NewAudioService() читает из неё стартовую громкость.
func setupPlayerTestDB(t *testing.T) {
	t.Helper()
	setupAudioTestDB(t)
	if err := inits.DB.AutoMigrate(&models.GlobalState{}); err != nil {
		t.Fatalf("automigrate GlobalState: %v", err)
	}
	inits.DB.FirstOrCreate(&models.GlobalState{}, models.GlobalState{})
}

// fakeSeekCloser — минимальный beep.StreamSeekCloser для тестов движка,
// которым не нужен настоящий аудиофайл: только факт, что Close() был
// вызван (для проверки И2/И3 — "старый src.Close() — после отпускания p.mu").
type fakeSeekCloser struct {
	onClose func()
}

func (f *fakeSeekCloser) Stream(samples [][2]float64) (int, bool) { return 0, false }
func (f *fakeSeekCloser) Err() error                              { return nil }
func (f *fakeSeekCloser) Len() int                                { return 0 }
func (f *fakeSeekCloser) Position() int                           { return 0 }
func (f *fakeSeekCloser) Seek(p int) error                        { return nil }
func (f *fakeSeekCloser) Close() error {
	if f.onClose != nil {
		f.onClose()
	}
	return nil
}

// TestPlayIsIdempotent — И5: повторный Play на уже играющий itemId должен
// быть no-op, а не рестартом. Проверяем это по прямому признаку рестарта —
// сброшенной позиции: startTrack() всегда обнуляет p.pos, значит если
// второй Play её не тронул, до него дело не дошло.
func TestPlayIsIdempotent(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = fakeOpenDevice(44100)

	src := filepath.Join(t.TempDir(), "track.wav")
	writeTestWav(t, src, 2)
	res := a.ImportTrack(context.Background(), src)
	if res.Error != "" {
		t.Fatalf("ImportTrack: %s", res.Error)
	}

	playlist := models.Playlist{Name: "Тест"}
	if err := inits.DB.Create(&playlist).Error; err != nil {
		t.Fatalf("create playlist: %v", err)
	}
	item := models.PlaylistItem{PlaylistId: playlist.ID, TrackId: res.Track.ID, Position: 1}
	if err := inits.DB.Create(&item).Error; err != nil {
		t.Fatalf("create playlist item: %v", err)
	}

	if err := a.Play(float32(playlist.ID), float32(item.ID)); err != nil {
		t.Fatalf("first Play: %v", err)
	}
	st := a.State()
	if st.Status != string(statusPlaying) {
		t.Fatalf("status = %q, want %q", st.Status, statusPlaying)
	}
	if st.TrackId != res.Track.ID {
		t.Fatalf("TrackId = %d, want %d", st.TrackId, res.Track.ID)
	}

	// Отметим "прогресс": настоящий рестарт (startTrack) обнулил бы это.
	a.pl.pos.Store(12345)

	if err := a.Play(float32(playlist.ID), float32(item.ID)); err != nil {
		t.Fatalf("second (repeat) Play: %v", err)
	}
	if got := a.pl.pos.Load(); got != 12345 {
		t.Errorf("repeat Play reset position to %d — treated as a restart, not a no-op (И5)", got)
	}
	st2 := a.State()
	if st2.TrackId != res.Track.ID || st2.ItemId != item.ID {
		t.Errorf("repeat Play changed current track: %+v", st2)
	}
}

// TestAudioServiceWithoutDevice — при недоступном устройстве вывода методы
// сервиса возвращают ошибку (или тихо ничего не делают там, где ошибка не
// нужна оператору), но не паникуют.
func TestAudioServiceWithoutDevice(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = func(string) (*malgo.AllocatedContext, *malgo.Device, uint32, string, error) {
		return nil, nil, 0, "", errors.New("нет звуковой карты")
	}

	if err := a.Play(1, 999999); err == nil {
		t.Error("Play без устройства должен вернуть ошибку, а не молча ничего не делать")
	}

	// Ничего из этого не должно паниковать, даже если ничего не играет и
	// устройства нет.
	a.Toggle()
	a.Stop()
	a.SetVolume(0.3)
	st := a.State()
	if st.Status != string(statusIdle) {
		t.Errorf("status = %q, want %q", st.Status, statusIdle)
	}
	if diff := st.Volume - 0.3; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("Volume = %v, want ~0.3 — SetVolume должен работать даже без устройства", st.Volume)
	}

	if err := a.Seek(0); err == nil {
		t.Error("Seek без активного трека должен вернуть ошибку, а не паниковать")
	}
}

// TestStopClosesSourceOutsideMuAndDropsStaleDone проверяет реальный
// вызывающий путь — AudioService.Stop(), а не p.stop() напрямую: та версия
// теста была тавтологичной для И2 (тест сам звал Close() и лишь проверял,
// что фейк сработал). Здесь fakeSeekCloser.Close пытается p.mu.TryLock():
// если бы Stop() закрывал src, всё ещё держа p.mu, TryLock вернул бы false
// и тест бы упал. Плюс И4: устаревшее поколение из done молча отбрасывается,
// не воскрешая уже остановленный трек.
func TestStopClosesSourceOutsideMuAndDropsStaleDone(t *testing.T) {
	p := newPlayer(1.0, "")
	a := &AudioService{pl: p}

	var closed atomic.Bool
	var closedUnderMu bool
	src := &fakeSeekCloser{onClose: func() {
		closed.Store(true)
		if !p.mu.TryLock() {
			closedUnderMu = true
			return
		}
		p.mu.Unlock()
	}}

	gen := p.nextGen()
	installed, _ := p.startTrack(gen, generators.Silence(-1), src, trackMeta{trackId: 1, itemId: 1, totalSamples: -1})
	if !installed {
		t.Fatal("startTrack should install the first chain")
	}

	a.Stop() // реальный путь AudioService, не p.stop() напрямую (И2)

	if !closed.Load() {
		t.Fatal("Stop() did not close the source")
	}
	if closedUnderMu {
		t.Error("src.Close() was called while p.mu was still held — И2 violation")
	}

	// "Трек доиграл" для того самого gen, который Stop() уже инвалидировал —
	// должно быть молча отброшено (И4).
	if _, _, _, changed := p.finishIfCurrent(gen); changed {
		t.Error("finishIfCurrent must drop a stale generation (И4), not resurrect a stopped track")
	}
}

// TestConcurrentSeekDoesNotRaceDecoder — тест-пуллер (И6: "вывод — интерфейс
// дай N фреймов"). Крутит p.onSamples в цикле в отдельной горутине, как
// это делал бы data-колбэк malgo, пока основная горутина непрерывно
// перематывает. Без snapshotForSeek/mix.Clear() ДО src.Seek() вне p.mu это
// ловится -race как конкурентный доступ к декодеру: onSamples держит p.mu
// на всём Stream(), но раньше src.Seek() в Seek() вызывался ВНЕ p.mu, пока
// старая цепочка (со ссылкой на тот же src) всё ещё была в микшере.
func TestConcurrentSeekDoesNotRaceDecoder(t *testing.T) {
	setupPlayerTestDB(t)
	a := NewAudioService()
	a.pl.openDevice = fakeOpenDevice(44100)

	src := filepath.Join(t.TempDir(), "track.wav")
	writeTestWav(t, src, 5)
	res := a.ImportTrack(context.Background(), src)
	if res.Error != "" {
		t.Fatalf("ImportTrack: %s", res.Error)
	}

	playlist := models.Playlist{Name: "Т"}
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

	var wg sync.WaitGroup
	stop := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 512*2*4)
		for {
			select {
			case <-stop:
				return
			default:
			}
			a.pl.onSamples(buf, nil, 512)
		}
	}()

	for i := 0; i < 300; i++ {
		ms := float32((i % 40) * 100)
		_ = a.Seek(ms) // ошибки (обогнали сами себя) допустимы, паники/гонки — нет
	}
	close(stop)
	wg.Wait()
}

// sineSrc — детерминированный тестовый источник для TestSeekRebuildNoStaleTail:
// значение в каждом кадре — чистая функция его абсолютного индекса, поэтому
// два независимо построенных sineSrc, оба перемотанные на одну и ту же
// позицию, обязаны дать бит-в-бит одинаковый выход всей цепочки. Это и
// отличает честную перемотку от хвоста старого места ресемплера.
type sineSrc struct {
	rate int
	freq float64
	pos  int
}

func (s *sineSrc) Stream(samples [][2]float64) (int, bool) {
	for i := range samples {
		v := math.Sin(2 * math.Pi * s.freq * float64(s.pos) / float64(s.rate))
		samples[i][0], samples[i][1] = v, v
		s.pos++
	}
	return len(samples), true
}
func (s *sineSrc) Err() error    { return nil }
func (s *sineSrc) Len() int      { return -1 }
func (s *sineSrc) Position() int { return s.pos }
func (s *sineSrc) Seek(p int) error {
	s.pos = p
	return nil
}
func (s *sineSrc) Close() error { return nil }

// TestSeekRebuildNoStaleTail — план, таблица тестов этапа 2: перемотка
// обязана пересобрать Resample+Volume заново, а не унести хвост старого
// места. Сравниваем цепочку "прогретую и перемотанную" с независимой
// "свежей, сразу построенной на позиции seekTo" — если реземплер честно не
// несёт памяти о том, что было до seekTo, оба выхода совпадут.
func TestSeekRebuildNoStaleTail(t *testing.T) {
	const rate = beep.SampleRate(8000)
	const seekTo = 3000

	p := newPlayer(1.0, "")
	p.devRate = rate
	src := &sineSrc{rate: int(rate), freq: 440}
	gen := p.nextGen()
	chain := buildChain(src, rate, rate, 0, -1, 0, p.done, gen)
	if installed, _ := p.startTrack(gen, chain, src, trackMeta{fileRate: rate, itemId: 1, totalSamples: -1}); !installed {
		t.Fatal("startTrack failed")
	}

	// "Прогреваем" — читаем несколько тысяч кадров, чтобы ресемплер накопил
	// внутреннее состояние (buf1/buf2/off/pos, resample.go:78-86), как за
	// секунды реального воспроизведения до того, как оператор потянет
	// ползунок.
	warm := make([][2]float64, 2000)
	if _, ok := p.mvol.Stream(warm); !ok {
		t.Fatal("warm-up stream failed")
	}

	snap, gen2, ok := p.snapshotForSeek()
	if !ok {
		t.Fatal("snapshotForSeek: nothing to seek")
	}
	if err := snap.src.Seek(seekTo); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	chain2 := buildChain(snap.src, snap.fileRate, snap.devRate, snap.gainDb, snap.totalSamples, snap.fadeSamples, p.done, gen2)
	if !p.resumeChain(gen2, chain2, snap.src, seekTo) {
		t.Fatal("resumeChain rejected a legitimate seek")
	}

	got := make([][2]float64, 32)
	if n, ok := p.mvol.Stream(got); !ok || n == 0 {
		t.Fatal("no samples after seek")
	}

	refP := newPlayer(1.0, "")
	refP.devRate = rate
	refSrc := &sineSrc{rate: int(rate), freq: 440}
	if err := refSrc.Seek(seekTo); err != nil {
		t.Fatalf("ref Seek: %v", err)
	}
	refGen := refP.nextGen()
	refChain := buildChain(refSrc, rate, rate, 0, -1, 0, refP.done, refGen)
	if installed, _ := refP.startTrack(refGen, refChain, refSrc, trackMeta{fileRate: rate, itemId: 1, totalSamples: -1}); !installed {
		t.Fatal("ref startTrack failed")
	}
	want := make([][2]float64, 32)
	if n, ok := refP.mvol.Stream(want); !ok || n == 0 {
		t.Fatal("ref: no samples")
	}

	for i := range got {
		if diff := got[i][0] - want[i][0]; diff > 1e-9 || diff < -1e-9 {
			t.Fatalf("sample %d = %v, want %v — stale resampler tail after seek", i, got[i][0], want[i][0])
		}
	}
}

// TestFadeOutStopDrains — план, этап 5, ГЛАВНЫЙ тест этапа. effects.Transition
// НИКОГДА не отдаёт ok=false сам (transition.go:54-70): Stream возвращает ok
// ИСТОЧНИКА, а после len продолжает тянуть его с endGain. Без внешнего
// beep.Take «Стоп с фейдом» увёл бы звук в тишину, а трек формально играл бы
// ещё минуты — audio_stopped не пришёл бы, следующий Play() наложился бы
// поверх. Строит ровно ту цепочку, что строит AudioService.FadeOutStop()
// (totalSamples == fadeSamples == вся оставшаяся жизнь цепочки — фейд), и
// требует ok=false НЕ ПОЗЖЕ fadeSamples сэмплов устройства.
//
// ⚠️ Читает chain.Stream() НАПРЯМУЮ, а не через p.mvol/p.mix: beep.Mixer
// убирает дренированный стример и, будучи пустым, дальше сам честно отдаёт
// тишину с ok=true (stopWhenEmpty=false по умолчанию, см. И6/комментарий у
// onSamples) — то есть замаскировал бы ровно тот баг, который этот тест
// обязан ловить.
func TestFadeOutStopDrains(t *testing.T) {
	const rate = beep.SampleRate(8000)
	const fadeMs = 250
	fadeSamples := deviceSamples(fadeMs, rate)

	src := &sineSrc{rate: int(rate), freq: 440}
	chain := buildChain(src, rate, rate, 0, fadeSamples, fadeSamples, make(chan uint64, 1), 1)

	buf := make([][2]float64, 37) // намеренно не делитель fadeSamples — ловит ошибки на границах чанков
	total := 0
	drained := false
	for i := 0; i < 10000; i++ {
		n, ok := chain.Stream(buf)
		total += n
		if !ok {
			drained = true
			break
		}
	}
	if !drained {
		t.Fatalf("цепочка ни разу не отдала ok=false за %d сэмплов — баг effects.Transition (никогда не отдаёт ok=false сам) не заблокирован внешним beep.Take", total)
	}
	if total > fadeSamples {
		t.Fatalf("отдренировалась после %d сэмплов, ожидалось не позже fadeSamples=%d", total, fadeSamples)
	}
}

// TestTrimTakesExactSamples — план, этап 5: "число сэмплов после ресемпла при
// заданных TrimStartMs/TrimEndMs". ⚠️ Частота файла (44100) сознательно
// ОТЛИЧАЕТСЯ от частоты устройства (48000): buildChain ставит Take ПОСЛЕ
// Resample, то есть считает в сэмплах УСТРОЙСТВА. Тест с одинаковыми частотами
// прошёл бы даже при перепутанных fileSamples/deviceSamples — главная ловушка
// этапа (audio-playlist-implementation.md, "Две частоты, две функции").
func TestTrimTakesExactSamples(t *testing.T) {
	const fileRate = beep.SampleRate(44100)
	const devRate = beep.SampleRate(48000)

	track := models.AudioTrack{DurationMs: 10000, TrimStartMs: 1000, TrimEndMs: 6000}
	if wantMs := trimmedDurationMs(track); wantMs != 5000 {
		t.Fatalf("trimmedDurationMs = %d, want 5000 (TrimEndMs-TrimStartMs)", wantMs)
	}
	totalSamples, fadeSamples := trackChainBounds(track, 0, devRate)
	if fadeSamples != 0 {
		t.Fatalf("fadeSamples = %d, want 0 (FadeMs=0)", fadeSamples)
	}
	wantSamples := deviceSamples(5000, devRate) // на частоте УСТРОЙСТВА, не файла
	if totalSamples != wantSamples {
		t.Fatalf("totalSamples = %d, want %d (deviceSamples(5000, devRate))", totalSamples, wantSamples)
	}

	src := &sineSrc{rate: int(fileRate), freq: 440}
	chain := buildChain(src, fileRate, devRate, 0, totalSamples, fadeSamples, make(chan uint64, 1), 1)

	// Читаем chain.Stream() напрямую — см. предупреждение у TestFadeOutStopDrains
	// про beep.Mixer, маскирующий ok=false пустотой.
	buf := make([][2]float64, 97) // не делитель totalSamples — ловит ошибки на границах чанков
	total := 0
	for i := 0; i < 100000; i++ {
		n, ok := chain.Stream(buf)
		total += n
		if total > totalSamples {
			t.Fatalf("поток не остановился на totalSamples=%d, дошёл до %d", totalSamples, total)
		}
		if !ok {
			break
		}
	}
	if total != totalSamples {
		t.Fatalf("отдано %d сэмплов устройства, ожидалось ровно %d", total, totalSamples)
	}
}

// TestZeroFadeProducesNoNaN — план, этап 5: FadeMs=0 обязан ПОЛНОСТЬЮ обходить
// effects.Transition, а не просто вызывать её с len=0. Внутри transition.go:59
// прогресс считается как pos/len — при len==0 это 0/0 = NaN, а min(NaN, 1.0) в
// Go тоже возвращает NaN: весь буфер сэмплов стал бы NaN, что на части
// звуковых драйверов звучит как громкий хлопок при каждом вызове Stream().
func TestZeroFadeProducesNoNaN(t *testing.T) {
	const rate = beep.SampleRate(8000)
	const total = 500 // граница есть (TrimEndMs), но фейда на ней нет (FadeMs=0)

	src := &sineSrc{rate: int(rate), freq: 440}
	chain := buildChain(src, rate, rate, 0, total, 0, make(chan uint64, 1), 1)

	buf := make([][2]float64, 64)
	streamed := 0
	for streamed < 600 {
		n, ok := chain.Stream(buf)
		for i := 0; i < n; i++ {
			if math.IsNaN(buf[i][0]) || math.IsNaN(buf[i][1]) {
				t.Fatalf("NaN сэмпл на позиции %d — FadeMs=0 обязан полностью обходить effects.Transition, а не звать её с len=0", streamed+i)
			}
		}
		streamed += n
		if !ok {
			break
		}
	}
}

// TestSeekRecomputesRemainingFromNewPosition — план, этап 5: beep.Take хранит
// remains int (compositors.go:20-23) и не идемпотентен. При пересборке
// цепочки после Seek totalSamples для buildChain обязан считаться как
// (исходный totalSamples − новая позиция), а не переиспользовать исходное
// значение — иначе после перемотки внутрь обрезанного окна трек либо не
// остановится вовремя, либо оборвётся раньше времени.
func TestSeekRecomputesRemainingFromNewPosition(t *testing.T) {
	const rate = beep.SampleRate(8000)
	const total = 4000  // окно после трима — 4000 сэмплов устройства
	const seekTo = 2500 // перематываем внутрь окна: до конца остаётся 1500

	p := newPlayer(1.0, "")
	p.devRate = rate
	src := &sineSrc{rate: int(rate), freq: 440}
	gen := p.nextGen()
	chain := buildChain(src, rate, rate, 0, total, 0, p.done, gen)
	if installed, _ := p.startTrack(gen, chain, src, trackMeta{fileRate: rate, itemId: 1, totalSamples: total}); !installed {
		t.Fatal("startTrack failed")
	}

	// "Прогреваем", как TestSeekRebuildNoStaleTail — читаем немного до seek.
	warm := make([][2]float64, 500)
	if _, ok := p.mvol.Stream(warm); !ok {
		t.Fatal("warm-up failed")
	}

	snap, gen2, ok := p.snapshotForSeek()
	if !ok {
		t.Fatal("snapshotForSeek: nothing to seek")
	}
	if err := snap.src.Seek(seekTo); err != nil {
		t.Fatalf("Seek: %v", err)
	}

	remaining, fade := trimFadeSamplesAt(snap.totalSamples, snap.fadeSamples, seekTo)
	wantRemaining := total - seekTo
	if remaining != wantRemaining {
		t.Fatalf("remaining = %d, want %d (total-seekTo, а не исходный total=%d)", remaining, wantRemaining, total)
	}

	chain2 := buildChain(snap.src, snap.fileRate, snap.devRate, snap.gainDb, remaining, fade, p.done, gen2)
	if !p.resumeChain(gen2, chain2, snap.src, int64(seekTo)) {
		t.Fatal("resumeChain rejected a legitimate seek")
	}

	// Читаем chain2.Stream() напрямую, а не через p.mvol/p.mix — см.
	// предупреждение у TestFadeOutStopDrains про beep.Mixer, маскирующий
	// ok=false пустотой после дренирования.
	buf := make([][2]float64, 33)
	got := 0
	for i := 0; i < 10000; i++ {
		n, ok := chain2.Stream(buf)
		got += n
		if got > wantRemaining {
			t.Fatalf("поток продолжается после %d сэмплов (ожидалось <= %d) — Take пересобран от ИСХОДНОГО total, а не от новой позиции", got, wantRemaining)
		}
		if !ok {
			break
		}
	}
	if got != wantRemaining {
		t.Fatalf("после seek отдано %d сэмплов, ожидалось ровно %d (total-newPos)", got, wantRemaining)
	}
}
