package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

// nativeExts — форматы, которые beep декодирует сам. Всё остальное
// конвертируется в FLAC при импорте (см. audio_import.go), чтобы в
// воскресенье плеер видел ровно четыре проверенных формата, а отказ по
// формату случился в будний день во время подготовки.
var nativeExts = map[string]bool{
	".mp3": true, ".wav": true, ".flac": true, ".ogg": true, ".oga": true,
}

// decodeExt открывает файл и возвращает потоковый декодер по явно
// переданному расширению (а не по расширению самого path — импортируемый
// файл временно лежит как <hash>.<ext>.part, и решать формат по расширению
// файла на диске означало бы всегда получать ".part"). Файл, с которого
// декодер построен успешно, остаётся у вызывающего — закрыть его обязан он
// же (streamer.Close()).
//
// ⚠️ На mp3 это полный проход по файлу: go-mp3 строит таблицу фреймов прямо в
// конструкторе декодера (ensureFrameStartsAndLength). Не считать дешёвым ни
// здесь, ни в будущем движке — ~9000 итераций на 4-минутной минусовке,
// ~300 000 на двухчасовой проповеди.
//
// ⚠️ На ошибке декодирования сам f закрывается здесь явно: ни mp3.Decode, ни
// vorbis.Decode не закрывают переданный io.ReadCloser, если конструктор
// декодера падает (сам файл битый, не тот формат под чужим расширением и
// т.п.) — они просто возвращают ошибку и оставляют файл открытым. На Windows
// это означает, что os.Remove на импорте невалидного файла провалился бы с
// sharing violation: .part остался бы на диске, а не удалился.
func decodeExt(path, ext string) (s beep.StreamSeekCloser, format beep.Format, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, beep.Format{}, err
	}

	switch strings.ToLower(ext) {
	case ".mp3":
		s, format, err = mp3.Decode(f)
	case ".wav":
		s, format, err = wav.Decode(f)
	case ".flac":
		s, format, err = flac.Decode(f)
	case ".ogg", ".oga":
		s, format, err = vorbis.Decode(f)
	default:
		err = fmt.Errorf("нет декодера для формата %s", ext)
	}
	if err != nil {
		f.Close()
		return nil, beep.Format{}, err
	}
	return s, format, nil
}
