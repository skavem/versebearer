//go:build windows

package inits

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// lockFileExclusive открывает path через Win32 CreateFile с dwShareMode=0 —
// ни чтение, ни запись, ни удаление другими хендлами не допускаются, пока
// возвращённая функция не закроет хендл. Это настоящая блокировка на уровне
// ОС (не файловый атрибут, который os.Remove на Windows умеет снимать сам),
// и она надёжно валит попытку SQLite открыть тот же файл — что на чтение
// (?mode=ro), что на чтение-запись.
func lockFileExclusive(t *testing.T, path string) (release func()) {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatalf("UTF16PtrFromString: %v", err)
	}
	h, err := windows.CreateFile(
		p,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, // без сharing — ни один другой хендл (включая свой процесс) не откроется
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		t.Fatalf("CreateFile(%s) exclusive: %v", path, err)
	}
	return func() { windows.CloseHandle(h) }
}

// TestMigrateCopyFailsWhenSourceIsLocked — реальный, не искусственный класс
// отказа копирования: исходный файл занят другой программой (антивирус,
// открытый в другом инструменте и т.п.) РОВНО в момент копирования — сам
// кандидат уже прошёл проверку "наша ли это база" секундой раньше (гонка:
// файл был свободен при отборе и занят к началу VACUUM INTO). Отсюда вызов
// copyLegacyDB напрямую, а не через migrateLegacyDB/findLegacyDB — тот путь,
// где блокировка стоит уже на этапе ОТБОРА кандидата, проверяет отдельный
// TestFindLegacyDBReturnsErrorOnLockedCandidate (HIGH-1 обзора): после
// исправления looksLikeOurDB такая блокировка больше не маскируется под
// "не наш файл", а валит миграцию явной *CandidateUnreadableError.
//
// openReadPreferRO пробует read-only, падает, пробует чтение-запись
// (Р6 — "восстановление горячего журнала"), тоже падает — эксклюзивная
// блокировка не пускает ни один из двух режимов. Копирование обязано
// вернуть *MigrationFailedError, не создать dst и не тронуть src.
func TestMigrateCopyFailsWhenSourceIsLocked(t *testing.T) {
	src := filepath.Join(t.TempDir(), "test.db")
	dst := filepath.Join(t.TempDir(), "versebearer.db")
	buildLegacySourceDB(t, src)

	srcInfoBefore, err := os.Stat(src)
	if err != nil {
		t.Fatalf("stat src: %v", err)
	}

	release := lockFileExclusive(t, src)
	defer release()

	_, err = copyLegacyDB(src, dst)
	if err == nil {
		t.Fatal("copyLegacyDB = nil, want error (источник заблокирован на уровне ОС)")
	}
	var migFailed *MigrationFailedError
	if !errors.As(err, &migFailed) {
		t.Fatalf("error = %v (%T), want *MigrationFailedError", err, err)
	}

	if _, statErr := os.Stat(dst); !os.IsNotExist(statErr) {
		t.Errorf("dst создан, хотя источник был заблокирован: stat err = %v", statErr)
	}
	srcInfoAfter, statErr := os.Stat(src)
	if statErr != nil {
		t.Fatalf("src исчез: %v", statErr)
	}
	if srcInfoAfter.Size() != srcInfoBefore.Size() {
		t.Errorf("src изменился в размере: было %d, стало %d", srcInfoBefore.Size(), srcInfoAfter.Size())
	}
	if matches, _ := filepath.Glob(dst + ".part.*"); len(matches) != 0 {
		t.Errorf("остался .part: %v", matches)
	}
}

// TestFindLegacyDBReturnsErrorOnLockedCandidate — HIGH-1 обзора. До правки
// looksLikeOurDB сливала "не смог проверить" (файл занят другой программой)
// с "точно не наш файл" — ошибка от gorm.Open превращалась в (false, nil), и
// findLegacyDB молча пропускал нечитаемого кандидата, как будто его вообще
// не было. Итог: живая test.db, которую в момент запуска держит эксклюзивным
// хендлом антивирус/незавершённый прошлый процесс, приводила к "чистой
// установке" — а на следующем запуске inspectDestination видит валидный
// SQLite на новом месте, и повторная миграция уже невозможна никогда.
// Правильное поведение: нечитаемый кандидат обязан вернуть ошибку, а не
// пропасть молча.
func TestFindLegacyDBReturnsErrorOnLockedCandidate(t *testing.T) {
	src := filepath.Join(t.TempDir(), "test.db")
	buildLegacySourceDB(t, src)

	release := lockFileExclusive(t, src)
	defer release()

	got, err := findLegacyDB([]string{src})
	if err == nil {
		t.Fatal("findLegacyDB = nil, want error (кандидат заблокирован на уровне ОС)")
	}
	if got != "" {
		t.Errorf("findLegacyDB вернул путь %q при ошибке — не должен", got)
	}
	var unreadable *CandidateUnreadableError
	if !errors.As(err, &unreadable) {
		t.Fatalf("error = %v (%T), want *CandidateUnreadableError", err, err)
	}
	if unreadable.Path != src {
		t.Errorf("CandidateUnreadableError.Path = %q, want %q", unreadable.Path, src)
	}
}
