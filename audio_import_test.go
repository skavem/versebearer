package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/paths"

	"github.com/adrg/xdg"
	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/generators"
	"github.com/gopxl/beep/v2/wav"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestMain points paths.MediaDir() at a fresh temp directory for the whole
// test binary. DataDir/MediaDir memoize their result behind sync.Once, so the
// env var has to be set before the very first call from any test in this
// package — hence doing it here rather than per-test.
func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "versebearer-audio-test-*")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_DATA_HOME", tmp)
	xdg.Reload()

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

func setupAudioTestDB(t *testing.T) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.AudioTrack{}, &models.Playlist{}, &models.PlaylistItem{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	inits.DB = db
}

// writeTestWav writes a minimal valid PCM wav file of silence — enough for
// decodeExt() to accept it as a real (if silent) track, without depending on
// any external fixture files.
func writeTestWav(t *testing.T, path string, seconds float64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create wav: %v", err)
	}
	defer f.Close()

	const sampleRate = beep.SampleRate(8000)
	format := beep.Format{SampleRate: sampleRate, NumChannels: 2, Precision: 2}
	numSamples := int(float64(sampleRate) * seconds)
	if err := wav.Encode(f, generators.Silence(numSamples), format); err != nil {
		t.Fatalf("encode wav: %v", err)
	}
}

func TestImportTrack_NativeFormatSucceeds(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	src := filepath.Join(t.TempDir(), "минусовка.wav")
	writeTestWav(t, src, 2)

	res := a.ImportTrack(context.Background(), src)
	if res.Error != "" {
		t.Fatalf("ImportTrack failed: %s", res.Error)
	}
	if res.Duplicate {
		t.Error("first import should not be flagged duplicate")
	}
	if res.Converted {
		t.Error("wav is native, should not be converted")
	}
	if res.Track == nil {
		t.Fatal("Track is nil")
	}
	if res.Track.DurationMs < 1900 || res.Track.DurationMs > 2100 {
		t.Errorf("DurationMs = %d, want ~2000", res.Track.DurationMs)
	}

	mediaDir, err := paths.MediaDir()
	if err != nil {
		t.Fatalf("MediaDir: %v", err)
	}
	finalPath := filepath.Join(mediaDir, res.Track.FileName)
	if _, err := os.Stat(finalPath); err != nil {
		t.Errorf("final file missing: %v", err)
	}
	if matches, _ := filepath.Glob(filepath.Join(mediaDir, "*.part")); len(matches) != 0 {
		t.Errorf(".part left behind: %v", matches)
	}
}

func TestImportTrack_DuplicateDetected(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	src := filepath.Join(t.TempDir(), "track.wav")
	writeTestWav(t, src, 1)

	first := a.ImportTrack(context.Background(), src)
	if first.Error != "" {
		t.Fatalf("first import failed: %s", first.Error)
	}

	second := a.ImportTrack(context.Background(), src)
	if second.Error != "" {
		t.Fatalf("second import failed: %s", second.Error)
	}
	if !second.Duplicate {
		t.Error("second import of the same bytes should be flagged duplicate")
	}
	if second.Track.ID != first.Track.ID {
		t.Error("duplicate should point at the original track")
	}

	var count int64
	inits.DB.Model(&models.AudioTrack{}).Count(&count)
	if count != 1 {
		t.Errorf("want 1 track row after duplicate import, got %d", count)
	}
}

func TestImportTrack_CorruptFileLeavesNoPart(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	src := filepath.Join(t.TempDir(), "not-really-audio.mp3")
	if err := os.WriteFile(src, []byte("this is not an mp3 file, just text pretending to be one"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := a.ImportTrack(context.Background(), src)
	if res.Error == "" {
		t.Fatal("importing a fake mp3 should fail")
	}

	// Media dir is shared across tests in this binary (paths.MediaDir() is
	// memoized) — assert only that THIS import left no .part behind, not that
	// the directory is empty (other tests populate it too).
	mediaDir, err := paths.MediaDir()
	if err != nil {
		t.Fatalf("MediaDir: %v", err)
	}
	if matches, _ := filepath.Glob(filepath.Join(mediaDir, "*.part")); len(matches) != 0 {
		t.Errorf(".part left behind after a failed import: %v", matches)
	}

	var count int64
	inits.DB.Model(&models.AudioTrack{}).Count(&count)
	if count != 0 {
		t.Errorf("failed import should not create a track row, got %d", count)
	}
}

func TestRemoveTrack_DeletesFileAndRow(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	src := filepath.Join(t.TempDir(), "track.wav")
	writeTestWav(t, src, 1)
	res := a.ImportTrack(context.Background(), src)
	if res.Error != "" {
		t.Fatalf("import failed: %s", res.Error)
	}

	mediaDir, _ := paths.MediaDir()
	finalPath := filepath.Join(mediaDir, res.Track.FileName)

	if err := a.RemoveTrack(float32(res.Track.ID)); err != nil {
		t.Fatalf("RemoveTrack: %v", err)
	}

	if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
		t.Errorf("file should be removed, stat err = %v", err)
	}
	var count int64
	inits.DB.Model(&models.AudioTrack{}).Count(&count)
	if count != 0 {
		t.Errorf("want 0 tracks after remove, got %d", count)
	}
}

func TestUpdateTrack_UpdatesOnlyGivenFields(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	src := filepath.Join(t.TempDir(), "track.wav")
	writeTestWav(t, src, 1)
	res := a.ImportTrack(context.Background(), src)
	if res.Error != "" {
		t.Fatalf("import failed: %s", res.Error)
	}

	// Задаём Artist/TrimStartMs/GainDb один раз, чтобы было что не потерять
	// на следующем, частичном апдейте.
	artist := "Автор"
	trimStart := 500
	gain := 3.5
	full, err := a.UpdateTrack(float32(res.Track.ID), TrackInput{
		Artist: &artist, TrimStartMs: &trimStart, GainDb: &gain,
	})
	if err != nil {
		t.Fatalf("UpdateTrack (full): %v", err)
	}
	if full.Artist != artist || full.TrimStartMs != trimStart || full.GainDb != gain {
		t.Fatalf("full update did not apply: %+v", full)
	}

	newTitle := "Новое название"
	updated, err := a.UpdateTrack(float32(res.Track.ID), TrackInput{Title: &newTitle})
	if err != nil {
		t.Fatalf("UpdateTrack (partial): %v", err)
	}
	if updated.Title != newTitle {
		t.Errorf("Title = %q, want %q", updated.Title, newTitle)
	}
	if updated.FileName != res.Track.FileName {
		t.Errorf("FileName changed unexpectedly: %q vs %q", updated.FileName, res.Track.FileName)
	}
	// Поля, не упомянутые во втором TrackInput{Title: ...}, обязаны остаться
	// такими же, как после первого апдейта — это и значит «только заданные
	// поля».
	if updated.Artist != artist {
		t.Errorf("Artist changed unexpectedly: %q, want %q", updated.Artist, artist)
	}
	if updated.TrimStartMs != trimStart {
		t.Errorf("TrimStartMs changed unexpectedly: %d, want %d", updated.TrimStartMs, trimStart)
	}
	if updated.GainDb != gain {
		t.Errorf("GainDb changed unexpectedly: %v, want %v", updated.GainDb, gain)
	}
}

func TestUpdateTrack_ErrorOnUnknownId(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	title := "x"
	if _, err := a.UpdateTrack(999999, TrackInput{Title: &title}); err == nil {
		t.Error("UpdateTrack on an unknown id should return an error, not silently do nothing")
	}
}

// TestImportTrack_ReusesPhantomRowWhenFileMissing — регрессия HIGH находки
// ревью: строка в БД может пережить свой файл (антивирус, ручная чистка
// %LOCALAPPDATA%, или RemoveTrack, у которого os.Remove файла прошёл, а
// последующее удаление строки — нет). Переимпорт того же исходника обязан
// заново создать файл и переиспользовать существующую строку, а не молча
// сообщить "уже импортировано" и ничего не сделать.
func TestImportTrack_ReusesPhantomRowWhenFileMissing(t *testing.T) {
	setupAudioTestDB(t)
	a := &AudioService{}

	src := filepath.Join(t.TempDir(), "track.wav")
	writeTestWav(t, src, 1)

	first := a.ImportTrack(context.Background(), src)
	if first.Error != "" {
		t.Fatalf("first import failed: %s", first.Error)
	}

	mediaDir, err := paths.MediaDir()
	if err != nil {
		t.Fatalf("MediaDir: %v", err)
	}
	if err := os.Remove(filepath.Join(mediaDir, first.Track.FileName)); err != nil {
		t.Fatalf("simulating file loss: %v", err)
	}

	second := a.ImportTrack(context.Background(), src)
	if second.Error != "" {
		t.Fatalf("reimport after file loss failed: %s", second.Error)
	}
	if second.Duplicate {
		t.Error("should not be flagged duplicate when the file is missing on disk")
	}
	if second.Track == nil || second.Track.ID != first.Track.ID {
		t.Errorf("expected reuse of the same row id, got %+v vs first id %d", second.Track, first.Track.ID)
	}

	finalPath := filepath.Join(mediaDir, second.Track.FileName)
	if _, err := os.Stat(finalPath); err != nil {
		t.Errorf("file should exist again after reimport: %v", err)
	}

	var count int64
	inits.DB.Model(&models.AudioTrack{}).Count(&count)
	if count != 1 {
		t.Errorf("want exactly 1 track row after reimport, got %d", count)
	}
}
