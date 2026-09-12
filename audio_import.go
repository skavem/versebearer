package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"changeme/backend/inits"
	"changeme/backend/models"
	"changeme/backend/paths"
)

// maxAudioImportFileSize — типичная фонограмма 5–80 МБ (см. план); предел
// ловит выбор явно не того файла, не типичный размер минусовки.
const maxAudioImportFileSize = 1 << 30 // 1 ГБ

// ffmpegTimeout ограничивает и конвертацию, и замер громкости — оба вызова
// внешнего процесса, оба должны иметь предел, иначе зависший ffmpeg подвесит
// импорт (конвертация — в критическом пути) или фоновую горутину (замер).
const ffmpegTimeout = 5 * time.Minute

// ImportTrackResult — результат импорта одного файла.
type ImportTrackResult struct {
	Track     *models.AudioTrack `json:"track"`
	Converted bool               `json:"converted"`
	Duplicate bool               `json:"duplicate"`
	Error     string             `json:"error"`
}

// canceledResult — общий ответ на отменённый ctx: ImportTrack проверяет отмену
// на каждой границе долгого шага (хеш, запись файла, декодирование), и
// формулировка у всех этих проверок обязана быть одна.
func canceledResult(ctx context.Context) *ImportTrackResult {
	if ctx.Err() == nil {
		return nil
	}
	return &ImportTrackResult{Error: "отменено"}
}

// audioFilePattern перечисляет форматы, которые вообще имеет смысл
// предлагать в диалоге: четыре нативных (см. nativeExts) плюс те, что beep не
// умеет, но ffmpeg сконвертирует при импорте (см. audio-playlist.md,
// «Форматы: гибридный импорт»).
const audioFilePattern = "*.mp3;*.wav;*.flac;*.ogg;*.oga;*.m4a;*.aac;*.wma;*.opus;*.aiff;*.aif;*.alac;*.wv"

// PickAudioFiles открывает диалог выбора файлов (можно несколько разом) и
// возвращает только пути. В отличие от PickTranslationFile (db_import.go),
// здесь нет отдельного шага чтения файла в память под превью — фонограмма
// весит десятки мегабайт, а не единицы, как перевод.
func (a *AudioService) PickAudioFiles() []string {
	if a.app == nil {
		return nil
	}

	dialog := a.app.Dialog.OpenFile()
	dialog.SetTitle("Выберите фонограммы")
	dialog.AddFilter("Аудио", audioFilePattern)
	dialog.CanChooseFiles(true)

	selected, err := dialog.PromptForMultipleSelection()
	if err != nil {
		log.Println("PickAudioFiles: dialog error", err)
		return nil
	}
	return selected
}

// ctxReader останавливает io.Copy, как только ctx отменён, — без него отмена
// импорта не доходила бы дальше следующей проверки ctx.Err() в ImportTrack, а
// hashSourceFile/copyFile гоняли бы io.Copy до конца на гигабайтном файле
// независимо от отмены.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

// hashSourceFile — sha256 исходника, первые 16 байт в hex. Даёт
// дедупликацию: тот же файл (те же байты), импортированный дважды или с
// разных флешек под разными именами, ложится в одну запись.
func hashSourceFile(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, ctxReader{ctx, f}); err != nil {
		return "", err
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum[:16]), nil
}

// findTrackByHash возвращает уже импортированный трек с таким же хешем
// исходника, если такой есть — независимо от того, жив ли ещё его файл на
// диске (это отдельно проверяет trackFileExists).
func findTrackByHash(hash string) *models.AudioTrack {
	var existing models.AudioTrack
	if err := inits.DB.Where("hash = ?", hash).First(&existing).Error; err != nil {
		return nil
	}
	return &existing
}

// trackFileExists проверяет, что файл трека действительно лежит в mediaDir.
// Без этой проверки совпадение по хешу считалось бы дубликатом даже для
// фантомной записи — строка есть, а файла на диске уже нет (антивирус,
// ручная чистка %LOCALAPPDATA%, или RemoveTrack, у которого os.Remove
// файла прошёл, а последующее удаление строки упало) — и переимпорт того же
// исходника молча ничего бы не делал.
func trackFileExists(mediaDir string, t *models.AudioTrack) bool {
	if t.FileName == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(mediaDir, t.FileName))
	return err == nil
}

// copyFile копирует native-формат в медиатеку как есть — в отличие от
// конвертации, здесь ffmpeg не участвует вовсе.
func copyFile(ctx context.Context, src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, ctxReader{ctx, in}); err != nil {
		return err
	}
	// Sync перед переименованием — весь смысл .part в защите от обрыва
	// (флешку выдернули, отключилось питание): без Sync данные могли ещё
	// сидеть в буфере ОС, когда Rename уже сделал файл "финальным" на вид.
	if err := out.Sync(); err != nil {
		return err
	}
	return out.Close()
}

// convertToPart перекодирует исходник в FLAC во временный .part. ffmpeg
// ищется здесь, а не на входе в ImportTrack: нативным форматам он не нужен
// вовсе, и его отсутствие не должно мешать им импортироваться.
func convertToPart(ctx context.Context, srcPath, partPath, srcExt string) error {
	ffmpeg, err := ffmpegPath()
	if err != nil {
		return fmt.Errorf("формат %s требует ffmpeg, а он не найден рядом с приложением", srcExt)
	}
	ctx, cancel := context.WithTimeout(ctx, ffmpegTimeout)
	defer cancel()
	return convertToFlac(ctx, ffmpeg, srcPath, partPath)
}

// saveTrackRow создаёт строку трека или обновляет фантомную (reuseId != 0 —
// строка пережила свой файл, см. trackFileExists). У фантомной обновляется
// только то, что действительно могло измениться: название, исполнителя, trim и
// громкость оператор мог править вручную, и переимпорт не должен их сбрасывать.
// Имя файла и хеш у неё те же по построению — то же содержимое всегда даёт то
// же <hash>.<ext>.
func saveTrackRow(reuseId uint, fresh models.AudioTrack) (*models.AudioTrack, error) {
	if reuseId == 0 {
		if err := inits.DB.Create(&fresh).Error; err != nil {
			return nil, fmt.Errorf("не удалось сохранить запись: %w", err)
		}
		return &fresh, nil
	}

	updates := map[string]any{
		"source_path": fresh.SourcePath,
		"duration_ms": fresh.DurationMs,
		"size_bytes":  fresh.SizeBytes,
	}
	if err := inits.DB.Model(&models.AudioTrack{}).Where("id = ?", reuseId).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("не удалось сохранить запись: %w", err)
	}
	var track models.AudioTrack
	if err := inits.DB.First(&track, reuseId).Error; err != nil {
		return nil, fmt.Errorf("не удалось прочитать обновлённую запись: %w", err)
	}
	return &track, nil
}

// ImportTrack копирует (или конвертирует) один файл в медиатеку.
//
// Последовательность — см. .omc/plans/audio-playlist-implementation.md, п. 1.5:
// хеш исходника → запись в <hash>.<ext>.part → декодирование .part как ворота
// валидации → перегон на финальное имя → строка в БД → фоновый замер
// громкости. Любая ошибка после создания .part удаляет его: на диске не
// должно оставаться огрызков под «правильным» именем, иначе повторный импорт
// того же файла после обрыва вернёт «уже импортировано», хотя на диске мусор.
//
// ctx — первым параметром: Wails v3 сам подставляет его и отдаёт фронту
// CancellablePromise (bindings.go: needsContext), поэтому у оператора есть
// настоящая отмена конвертации конкретного файла, а не только прекращение
// цикла перед следующим — единственный по-настоящему долгий шаг здесь.
func (a *AudioService) ImportTrack(ctx context.Context, path string) *ImportTrackResult {
	if res := canceledResult(ctx); res != nil {
		return res
	}

	info, err := os.Stat(path)
	if err != nil {
		return &ImportTrackResult{Error: "файл не найден"}
	}
	if info.Size() > maxAudioImportFileSize {
		return &ImportTrackResult{Error: "файл слишком большой (больше 1 ГБ)"}
	}

	hash, err := hashSourceFile(ctx, path)
	if err != nil {
		return &ImportTrackResult{Error: "не удалось прочитать файл: " + err.Error()}
	}

	mediaDir, err := paths.MediaDir()
	if err != nil {
		return &ImportTrackResult{Error: "не удалось подготовить каталог медиатеки: " + err.Error()}
	}

	// Дубликат по хешу — только если файл действительно на месте.
	// Фантомную запись (строка есть, файла нет) чинить в этом этапе больше
	// нечем, кроме как переиспользовать её id при следующем импорте того же
	// исходника — см. trackFileExists.
	var reuseId uint
	if existing := findTrackByHash(hash); existing != nil {
		if trackFileExists(mediaDir, existing) {
			return &ImportTrackResult{Track: existing, Duplicate: true}
		}
		reuseId = existing.ID
	}

	if res := canceledResult(ctx); res != nil {
		return res
	}

	srcExt := strings.ToLower(filepath.Ext(path))
	converted := !nativeExts[srcExt]
	finalExt := srcExt
	if converted {
		finalExt = ".flac"
	}
	finalName := hash + finalExt
	finalPath := filepath.Join(mediaDir, finalName)
	partPath := finalPath + ".part"

	// committed становится true только после успешного os.Rename. До этого
	// момента ЛЮБОЙ выход из функции (ошибка, отмена, паника — defer
	// выполняется и при панике) обязан подчистить .part, иначе на диске
	// останется огрызок под "правильным" именем (план, п. 1.5, шаг 4).
	committed := false
	defer func() {
		if !committed {
			os.Remove(partPath)
		}
	}()

	if converted {
		if err := convertToPart(ctx, path, partPath, srcExt); err != nil {
			return &ImportTrackResult{Error: err.Error()}
		}
	} else {
		if err := copyFile(ctx, path, partPath); err != nil {
			return &ImportTrackResult{Error: "не удалось скопировать файл: " + err.Error()}
		}
	}

	if res := canceledResult(ctx); res != nil {
		return res
	}

	// Декодирование .part — не побочный шаг ради длительности, а ворота
	// валидации: файл с чужим содержимым под "правильным" расширением
	// (текстовик, переименованный в .mp3) отсеивается здесь, а не в
	// воскресенье на сцене. decodeExt, а не decode: сам .part лежит под
	// именем "<hash>.<ext>.part", и решать формат по расширению файла на
	// диске означало бы всегда получать ".part".
	streamer, format, err := decodeExt(partPath, finalExt)
	if err != nil {
		return &ImportTrackResult{Error: "файл повреждён или не является аудио: " + err.Error()}
	}
	// sync.OnceFunc — Close() обязан быть отложен для безопасности на панике
	// (гипотетической: сейчас ниже её не осталось, но следующая правка может
	// её вернуть), и ОДНОВРЕМЕННО вызван явно ДО os.Rename: на Windows нельзя
	// переименовать файл с открытым хендлом ("used by another process").
	// Двойного вызова Close() эта защита не боится — все три декодера просто
	// закрывают os.File, а его повторный Close() возвращает ошибку, а не
	// паникует.
	closeStreamer := sync.OnceFunc(func() { streamer.Close() })
	defer closeStreamer()

	// FLAC легально несёт SampleRate=0 в STREAMINFO (значение "не указано") —
	// без этой проверки деление ниже паникует прямо в методе Wails-сервиса.
	if format.SampleRate <= 0 {
		return &ImportTrackResult{Error: "не удалось определить частоту дискретизации файла"}
	}
	// ⚠️ У MP3 длительность здесь завышена на ~64 мс — паддинг, который
	// кодировщик добавляет и который go-mp3 включает в общее число сэмплов.
	// На слух неважно, но не считать точной величиной, если где-то
	// понадобится точный «осталось».
	durationMs := streamer.Len() * 1000 / int(format.SampleRate)

	if res := canceledResult(ctx); res != nil {
		return res
	}

	// Закрыть ДО Rename, не после: пока декодер держит .part открытым,
	// переименование на Windows падает с sharing violation.
	closeStreamer()

	if err := os.Rename(partPath, finalPath); err != nil {
		return &ImportTrackResult{Error: "не удалось сохранить файл: " + err.Error()}
	}
	committed = true

	var sizeBytes int64
	if finalInfo, err := os.Stat(finalPath); err == nil {
		sizeBytes = finalInfo.Size()
	}

	track, err := saveTrackRow(reuseId, models.AudioTrack{
		Title:      strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		FileName:   finalName,
		DurationMs: durationMs,
		SizeBytes:  sizeBytes,
		SourcePath: path,
		Hash:       hash,
	})
	if err != nil {
		return &ImportTrackResult{Error: err.Error()}
	}

	a.emit("audio_tracks_update", a.ListTracks())

	// Замер громкости — фоново, после создания строки: 4-минутный трек
	// занимает у ffmpeg единицы секунд, но 50 файлов в массовом импорте на
	// синхронном замере превратились бы в минуты без возможности отмены.
	go a.measureAndStoreGain(track.ID, finalPath)

	return &ImportTrackResult{Track: track, Converted: converted}
}

// measureAndStoreGain замеряет громкость уже импортированного трека и
// сохраняет поправку. Нет ffmpeg или ошибка измерения — GainDb остаётся 0
// (тот же результат, что и осознанный отказ от нормализации), импорт уже
// считается успешным независимо от этого шага.
func (a *AudioService) measureAndStoreGain(trackId uint, filePath string) {
	ffmpeg, err := ffmpegPath()
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), ffmpegTimeout)
	defer cancel()

	gain, err := measureGainDb(ctx, ffmpeg, filePath)
	if err != nil {
		a.emit("audio_error", fmt.Sprintf("не удалось измерить громкость: %s", err.Error()))
		return
	}

	if err := inits.DB.Model(&models.AudioTrack{}).Where("id = ?", trackId).Update("gain_db", gain).Error; err != nil {
		log.Println("measureAndStoreGain: error saving gain", err)
		// Асинхронный путь, production-бинарь без консоли (-H windowsgui) —
		// log.Println здесь никто никогда не увидит. Ошибка измерения уже
		// шлёт audio_error несколькими строками выше; ошибка сохранения
		// результата не менее важна оператору, чем сама измеренная громкость.
		a.emit("audio_error", fmt.Sprintf("не удалось сохранить измеренную громкость: %s", err.Error()))
		return
	}
	a.emit("audio_tracks_update", a.ListTracks())
}
