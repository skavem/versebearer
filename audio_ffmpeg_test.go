package main

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Реальный вид stderr ffmpeg для `-af loudnorm=print_format=json -f null -`:
// баннер, "Input #0", "Stream mapping", строки прогресса, и только в конце —
// блок JSON. Прямой json.Unmarshal всего этого текста упал бы.
const loudnormStderrFixture = `ffmpeg version 7.1.1 Copyright (c) 2000-2025 the FFmpeg developers
  built with gcc 13.2.0
Input #0, wav, from 'quiet.wav':
  Duration: 00:00:04.00, bitrate: 1411 kb/s
  Stream #0:0: Audio: pcm_s16le, 44100 Hz, stereo, s16, 1411 kb/s
Stream mapping:
  Stream #0:0 -> #0:0 (pcm_s16le (native) -> pcm_s16le (native))
Press [q] to stop, [?] for help
size=N/A time=00:00:04.00 bitrate=N/A speed= 154x
[Parsed_loudnorm_0 @ 0x55d1a2b3f6c0]
{
	"input_i" : "-23.71",
	"input_tp" : "-6.54",
	"input_lra" : "8.40",
	"input_thresh" : "-34.02",
	"output_i" : "-24.13",
	"output_tp" : "-2.02",
	"output_lra" : "7.20",
	"output_thresh" : "-34.46",
	"normalization_type" : "dynamic",
	"target_offset" : "0.13"
}
`

const loudnormStderrSilenceFixture = `ffmpeg version 7.1.1 Copyright (c) 2000-2025 the FFmpeg developers
Input #0, wav, from 'silence.wav':
  Duration: 00:00:02.00, bitrate: 1411 kb/s
[Parsed_loudnorm_0 @ 0x55d1a2b3f6c0]
{
	"input_i" : "-inf",
	"input_tp" : "-inf",
	"input_lra" : "0.00",
	"input_thresh" : "-inf",
	"output_i" : "-70.00",
	"output_tp" : "-70.00",
	"output_lra" : "0.00",
	"output_thresh" : "-80.00",
	"normalization_type" : "dynamic",
	"target_offset" : "0.00"
}
`

func TestExtractLoudnormStats(t *testing.T) {
	stats, err := extractLoudnormStats(loudnormStderrFixture)
	if err != nil {
		t.Fatalf("extractLoudnormStats: %v", err)
	}
	if stats.InputI != "-23.71" || stats.InputTP != "-6.54" {
		t.Errorf("unexpected stats: %+v", stats)
	}
}

func TestExtractLoudnormStats_NoJSON(t *testing.T) {
	if _, err := extractLoudnormStats("no json here at all"); err == nil {
		t.Fatal("want error when there is no JSON block")
	}
}

func TestParseLoudnormFloat_HandlesInf(t *testing.T) {
	if v := parseLoudnormFloat("-inf"); !math.IsInf(v, -1) {
		t.Errorf("parseLoudnormFloat(-inf) = %v, want -Inf", v)
	}
	if v := parseLoudnormFloat("-23.71"); v != -23.71 {
		t.Errorf("parseLoudnormFloat(-23.71) = %v", v)
	}
	if v := parseLoudnormFloat("garbage"); !math.IsInf(v, -1) {
		t.Errorf("parseLoudnormFloat(garbage) = %v, want -Inf (defensive)", v)
	}
}

func TestComputeGainDb_Silence(t *testing.T) {
	stats, err := extractLoudnormStats(loudnormStderrSilenceFixture)
	if err != nil {
		t.Fatalf("extractLoudnormStats: %v", err)
	}
	inputI := parseLoudnormFloat(stats.InputI)
	inputTP := parseLoudnormFloat(stats.InputTP)

	// input_i/input_tp = -inf делает want/cap = +Inf — без явной защиты
	// clamp(+Inf, -12, 12) дал бы +12 дБ на пустом файле.
	if got := computeGainDb(inputI, inputTP); got != 0 {
		t.Errorf("computeGainDb(silence) = %v, want 0", got)
	}
}

func TestComputeGainDb_QuietTrack(t *testing.T) {
	stats, err := extractLoudnormStats(loudnormStderrFixture)
	if err != nil {
		t.Fatalf("extractLoudnormStats: %v", err)
	}
	inputI := parseLoudnormFloat(stats.InputI)   // -23.71
	inputTP := parseLoudnormFloat(stats.InputTP) // -6.54

	// want = -16 - (-23.71) = 7.71; cap = -1 - (-6.54) = 5.54 -> min = 5.54
	got := computeGainDb(inputI, inputTP)
	want := 5.54
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("computeGainDb = %v, want %v", got, want)
	}
}

func TestComputeGainDb_ClampsToPlusMinus12(t *testing.T) {
	// input_i очень тихий, но и пик далеко от нуля — cap не мешает, want
	// зашкаливает за 12.
	if got := computeGainDb(-40, -40); got != 12 {
		t.Errorf("computeGainDb = %v, want clamp to 12", got)
	}
	// input_i очень громкий — нужна большая отрицательная поправка.
	if got := computeGainDb(10, 10); got != -12 {
		t.Errorf("computeGainDb = %v, want clamp to -12", got)
	}
}

func TestComputeGainDb_PeakCapProtectsFromClipping(t *testing.T) {
	// Тихий по интегральной громкости трек, но пики почти у нуля — без
	// потолка по пику коррекция ушла бы в клиппинг.
	inputI, inputTP := -30.0, -0.2
	got := computeGainDb(inputI, inputTP)
	want := peakHeadroomDb - inputTP // -1 - (-0.2) = -0.8, меньше чем want=14
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("computeGainDb = %v, want peak-capped %v", got, want)
	}
}

// TestComputeGainDb_GatedLoudnessOnly ловит регрессию найденную ревью: loudnorm
// гейтит input_i, но не input_tp, поэтому (-inf, конечное) — штатный вывод
// для любого тихого материала, а не только для полной тишины (где -inf идёт
// в обоих полях, как в loudnormStderrSilenceFixture). Старая реализация
// проверяла math.IsInf ПОСЛЕ math.Min и здесь бы молча взяла конечный peakCap,
// вернув +12 дБ на почти тихом треке.
func TestComputeGainDb_GatedLoudnessOnly(t *testing.T) {
	if got := computeGainDb(math.Inf(-1), -98.03); got != 0 {
		t.Errorf("computeGainDb(-inf, -98.03) = %v, want 0", got)
	}
	// И симметрично: конечная громкость, но полностью цифровая тишина по
	// пику (не должно случаться на практике, но защита обязана быть общей).
	if got := computeGainDb(-23.71, math.Inf(-1)); got != 0 {
		t.Errorf("computeGainDb(-23.71, -inf) = %v, want 0", got)
	}
}

// ffmpegExeForTest ищет ffmpeg для реального прогона конвертации: сначала
// там, куда его кладёт fetch:ffmpeg (build/windows/ffmpeg), затем в PATH.
// Никак не переиспользует ffmpegPath() — та привязана к каталогу тестового
// бинаря, где ffmpeg никогда не лежит.
func ffmpegExeForTest() string {
	if abs, err := filepath.Abs(filepath.Join("build", "windows", "ffmpeg", ffmpegExeName())); err == nil {
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}
	if p, err := exec.LookPath(ffmpegExeName()); err == nil {
		return p
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	return ""
}

// TestConvertToFlac_RealFFmpeg — регрессионный тест на CRITICAL находку
// ревью: без "-f flac" ffmpeg отказывался писать в файл с "неправильным"
// (для выбора мультиплексора) расширением "*.flac.part" — ровно то имя, под
// которым конвертация реально идёт в ImportTrack. Скипается, если ffmpeg не
// нашёлся, но на машине с ffmpeg реально гоняет конвертацию и декодирует
// результат.
func TestConvertToFlac_RealFFmpeg(t *testing.T) {
	ffmpeg := ffmpegExeForTest()
	if ffmpeg == "" {
		t.Skip("ffmpeg не найден — пропускаем тест реальной конвертации")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "src.wav")
	writeTestWav(t, src, 1)

	// То самое "неудобное" имя: <hash>.flac.part.
	dst := filepath.Join(dir, "8346843df7f31978.flac.part")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := convertToFlac(ctx, ffmpeg, src, dst); err != nil {
		t.Fatalf("convertToFlac: %v", err)
	}

	streamer, format, err := decodeExt(dst, ".flac")
	if err != nil {
		t.Fatalf("decoding the converted file failed: %v", err)
	}
	defer streamer.Close()

	if format.SampleRate <= 0 {
		t.Errorf("unexpected sample rate: %d", format.SampleRate)
	}
	if streamer.Len() <= 0 {
		t.Error("converted file has no samples")
	}
}
