// Package paths закладывает будущее место пользовательских данных
// (фонограммы, а позже — база и поисковый индекс), не трогая то, что уже
// есть: test.db и search.bleve сейчас остаются в рабочем каталоге, перенос —
// отдельная задача.
package paths

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/adrg/xdg"
)

// appDirName — подкаталог внутри пользовательского каталога данных. На
// Windows xdg.DataHome — это %LOCALAPPDATA%, а не %APPDATA% (роуминг):
// фонограммы весят десятки мегабайт, и в роуминге они синхронизировались бы
// на каждый вход в домен.
const appDirName = "versebearer"

var (
	dataDirOnce sync.Once
	dataDirVal  string
	dataDirErr  error

	mediaDirOnce sync.Once
	mediaDirVal  string
	mediaDirErr  error
)

// DataDir возвращает (и при первом обращении создаёт) корень пользовательских
// данных приложения.
func DataDir() (string, error) {
	dataDirOnce.Do(func() {
		dir := filepath.Join(xdg.DataHome, appDirName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			dataDirErr = err
			return
		}
		dataDirVal = dir
	})
	return dataDirVal, dataDirErr
}

// MediaDir возвращает (и при первом обращении создаёт) каталог с
// импортированными фонограммами.
func MediaDir() (string, error) {
	mediaDirOnce.Do(func() {
		root, err := DataDir()
		if err != nil {
			mediaDirErr = err
			return
		}
		dir := filepath.Join(root, "media")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			mediaDirErr = err
			return
		}
		mediaDirVal = dir
	})
	return mediaDirVal, mediaDirErr
}

// DBPath — будущее место versebearer.db. Пока НЕ используется:
// backend/inits/db.go по-прежнему открывает "test.db" в рабочем каталоге.
// Перенос существующей базы (с миграцией файла) — отдельная задача; здесь
// только закреплено итоговое имя, чтобы не изобретать его заново позже.
func DBPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "versebearer.db"), nil
}
