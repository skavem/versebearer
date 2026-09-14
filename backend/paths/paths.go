// Package paths — единственный источник истины о размещении пользовательских
// данных приложения: фонограмм, базы SQLite и поискового индекса Bleve. Все
// три лежат в одном каталоге ($DataDir), который по умолчанию — подкаталог
// пользовательского каталога данных ОС, а на время разработки/тестов может
// быть переопределён переменной окружения VERSEBEARER_DATA (см. DataDir).
package paths

import (
	"os"
	"path/filepath"
	"strings"
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
//
// Каталог читается из VERSEBEARER_DATA, если переменная задана непустой
// строкой (после TrimSpace); appDirName к её значению НЕ приписывается —
// переменная называет каталог данных приложения целиком, а не его родителя
// (VERSEBEARER_DATA=D:\vb-dev даёт D:\vb-dev\versebearer.db, а не
// D:\vb-dev\versebearer\versebearer.db). Не задана — используется
// xdg.DataHome/appDirName.
//
// Значение кешируется в sync.Once и вычисляется ровно один раз за жизнь
// процесса: часть программы могла уже записать файлы по прежнему пути, так
// что перевычисление на лету было бы опаснее, чем полезно. Кто выставляет
// переменную, обязан сделать это до первого обращения к DataDir — процессное
// окружение (Taskfile) или тестовый TestMain/t.Setenv. Экспортируемого
// Reset() нет намеренно; в тестах пакета сброс sync.Once делается напрямую
// (см. paths_test.go) — это внутрипакетная механика, не публичный API.
func DataDir() (string, error) {
	dataDirOnce.Do(func() {
		dir := strings.TrimSpace(os.Getenv("VERSEBEARER_DATA"))
		if dir == "" {
			dir = filepath.Join(xdg.DataHome, appDirName)
		}
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

// DBPath — путь к файлу SQLite-базы внутри DataDir.
func DBPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "versebearer.db"), nil
}

// SearchIndexPath — путь к каталогу поискового индекса Bleve внутри DataDir.
// Отдельный sync.Once не нужен: каталог создаёт сам Bleve при первом
// открытии, а сюда достаточно DataDir() + join.
func SearchIndexPath() (string, error) {
	dir, err := DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "search.bleve"), nil
}
