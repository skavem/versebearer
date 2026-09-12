package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ffmpegExeName — имя исполняемого файла ffmpeg на текущей ОС.
func ffmpegExeName() string {
	if runtime.GOOS == "windows" {
		return "ffmpeg.exe"
	}
	return "ffmpeg"
}

// ffmpegPath ищет ffmpeg ТОЛЬКО рядом с исполняемым файлом приложения.
// Системный ffmpeg оператора из PATH сознательно игнорируется: его версия и
// набор кодеков нам неизвестны, а импорт должен вести себя одинаково на любой
// машине. На macOS/Linux ffmpeg рядом с бинарём не поставляется (см.
// build/AGENTS.md) — там эта функция всегда возвращает ошибку, и импорт
// нативных форматов (mp3/wav/flac/ogg) продолжает работать без него.
func ffmpegPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("не удалось определить путь к приложению: %w", err)
	}
	path := filepath.Join(filepath.Dir(exe), ffmpegExeName())
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("ffmpeg не найден рядом с приложением")
	}
	return path, nil
}

// runFFmpeg запускает ffmpeg с заданными аргументами и возвращает его stderr
// (ffmpeg пишет туда и прогресс, и ошибки, и, для -af loudnorm, сам результат
// измерения — stdout не используется вовсе).
func runFFmpeg(ctx context.Context, ffmpeg string, args ...string) (stderr string, err error) {
	cmd := exec.CommandContext(ctx, ffmpeg, args...)
	hideWindow(cmd)
	var buf bytes.Buffer
	cmd.Stderr = &buf
	runErr := cmd.Run()
	return buf.String(), runErr
}

// convertToFlac перекодирует src в FLAC без потерь — путь для форматов, которые
// beep не декодирует нативно (см. nativeExts в audio_decode.go).
//
// ⚠️ -f flac обязателен. dst — это <hash>.flac.part: ffmpeg выбирает
// мультиплексор по расширению ПОСЛЕ ПОСЛЕДНЕЙ ТОЧКИ, то есть по "part",
// которого в реестре форматов нет, и без -f падает с "Unable to choose an
// output format". -c:a flac сама по себе выбор контейнера не определяет.
func convertToFlac(ctx context.Context, ffmpeg, src, dst string) error {
	stderr, err := runFFmpeg(ctx, ffmpeg,
		"-nostdin", "-y", "-i", src, "-map", "0:a:0", "-c:a", "flac", "-compression_level", "5", "-f", "flac", dst,
	)
	if err != nil {
		return fmt.Errorf("ffmpeg не смог конвертировать файл: %w (%s)", err, lastLine(stderr))
	}
	return nil
}

// targetLUFS — цель нормализации громкости (EBU R128, вещательный стандарт).
const targetLUFS = -16.0

// peakHeadroomDb — запас до клиппинга по истинному пику.
const peakHeadroomDb = -1.0

// measureGainDb замеряет громкость файла через ffmpeg loudnorm и возвращает
// поправку в дБ для применения на воспроизведении (см. GainDb в
// models.AudioTrack). Файл не перекодируется — только читается.
func measureGainDb(ctx context.Context, ffmpeg, path string) (float64, error) {
	// -map 0:a:0 -vn: mp3 со встроенной обложкой несёт её как отдельный
	// video-поток (attached_pic) — без явного отбора аудио loudnorm рискует
	// получить на вход не то. -nostdin — на случай, если процесс когда-нибудь
	// запустят не в HideWindow-режиме, ffmpeg не должен виснуть на stdin.
	stderr, runErr := runFFmpeg(ctx, ffmpeg,
		"-nostdin", "-i", path, "-map", "0:a:0", "-vn", "-af", "loudnorm=print_format=json", "-f", "null", "-",
	)
	// ffmpeg с -f null обычно завершается кодом 0 даже когда что-то пошло не
	// так по существу измерения — проверяем не код возврата, а сам факт, что
	// JSON нашёлся; runErr тут почти всегда nil, но на случай отсутствия
	// самого бинаря/битого файла даём осмысленную ошибку.
	stats, parseErr := extractLoudnormStats(stderr)
	if parseErr != nil {
		if runErr != nil {
			return 0, fmt.Errorf("ffmpeg: %w", runErr)
		}
		return 0, parseErr
	}

	inputI := parseLoudnormFloat(stats.InputI)
	inputTP := parseLoudnormFloat(stats.InputTP)
	return computeGainDb(inputI, inputTP), nil
}

// computeGainDb — чистая часть measureGainDb, вынесенная отдельно ради теста
// без реального ffmpeg под рукой.
//
// ⚠️ Проверять входы нужно ПО ОТДЕЛЬНОСТИ и ДО арифметики. loudnorm гейтит
// input_i (интегральную громкость по стандарту гейтинга EBU R128), но НЕ
// input_tp (истинный пик) — пара (input_i="-inf", input_tp=конечное) штатный
// вывод для любого тихого материала, не только для полной цифровой тишины.
// Старая версия проверяла math.IsInf(gain, 0) ПОСЛЕ math.Min: при want=+Inf
// (из -inf input_i) и конечном peakCap min молча брал конечный peakCap, и
// защита не срабатывала вовсе — только когда ОБА поля были -inf.
func computeGainDb(inputI, inputTP float64) float64 {
	if !isFiniteDb(inputI) || !isFiniteDb(inputTP) {
		return 0
	}
	want := targetLUFS - inputI
	peakCap := peakHeadroomDb - inputTP
	return clampDb(math.Min(want, peakCap))
}

func isFiniteDb(v float64) bool {
	return !math.IsInf(v, 0) && !math.IsNaN(v)
}

func clampDb(v float64) float64 {
	if v < -12 {
		return -12
	}
	if v > 12 {
		return 12
	}
	return v
}

// loudnormStats — подмножество JSON, который ffmpeg печатает в stderr после
// прохода -af loudnorm=print_format=json. Поля намеренно строки: сам ffmpeg
// печатает числа в кавычках, а тишина даёт буквально "-inf".
type loudnormStats struct {
	InputI  string `json:"input_i"`
	InputTP string `json:"input_tp"`
}

// extractLoudnormStats вырезает из stderr ffmpeg последний блок от '{' до
// '}' и разбирает его как JSON. До этого блока в stderr идут баннер,
// "Input #0", "Stream mapping" и строки прогресса — прямой json.Unmarshal
// всего вывода упал бы.
func extractLoudnormStats(stderr string) (*loudnormStats, error) {
	start := strings.LastIndex(stderr, "{")
	end := strings.LastIndex(stderr, "}")
	if start < 0 || end < start {
		return nil, fmt.Errorf("не найден результат loudnorm в выводе ffmpeg")
	}
	var stats loudnormStats
	if err := json.Unmarshal([]byte(stderr[start:end+1]), &stats); err != nil {
		return nil, fmt.Errorf("не удалось разобрать результат loudnorm: %w", err)
	}
	return &stats, nil
}

// parseLoudnormFloat разбирает строковое поле loudnorm-JSON. strconv.ParseFloat
// сам понимает "-inf"/"inf" (регистронезависимо) — ровно то значение, которое
// ffmpeg пишет для полной тишины. Нечисловой мусор трактуется как -inf, чтобы
// он не улетел в +12 дБ, а сработала та же защита, что и для настоящей тишины.
func parseLoudnormFloat(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return math.Inf(-1)
	}
	return v
}

// lastLine возвращает последнюю непустую строку — короткий хвост stderr для
// сообщения об ошибке вместо десятков строк ffmpeg-баннера целиком.
func lastLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n\r"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" {
			return line
		}
	}
	return ""
}
