package paths

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/adrg/xdg"
)

// withTempDataHome points XDG_DATA_HOME at a fresh temp dir and resets the
// package-level sync.Once guards so DataDir/MediaDir recompute against it —
// otherwise the first caller in the test binary would have already cached a
// different value for the whole process.
func withTempDataHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmp)
	// Пустая строка после TrimSpace в DataDir() читается как "не задана" —
	// без этого сброса VERSEBEARER_DATA, унаследованная из окружения задачи
	// test (см. Taskfile.yml), победила бы xdg и все тесты в этом хелпере
	// стали бы проверять не то, что заявлено.
	t.Setenv("VERSEBEARER_DATA", "")
	xdg.Reload()

	dataDirOnce = sync.Once{}
	dataDirVal, dataDirErr = "", nil
	mediaDirOnce = sync.Once{}
	mediaDirVal, mediaDirErr = "", nil

	return tmp
}

func TestDataDir_CreatesUnderXDGDataHome(t *testing.T) {
	tmp := withTempDataHome(t)

	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	want := filepath.Join(tmp, "versebearer")
	if dir != want {
		t.Errorf("DataDir = %q, want %q", dir, want)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("DataDir did not create the directory: %v", err)
	}
}

func TestMediaDir_NestedUnderDataDir(t *testing.T) {
	tmp := withTempDataHome(t)

	media, err := MediaDir()
	if err != nil {
		t.Fatalf("MediaDir: %v", err)
	}
	want := filepath.Join(tmp, "versebearer", "media")
	if media != want {
		t.Errorf("MediaDir = %q, want %q", media, want)
	}
	if info, err := os.Stat(media); err != nil || !info.IsDir() {
		t.Errorf("MediaDir did not create the directory: %v", err)
	}
}

// TestDataDirRespectsEnv — Р10: VERSEBEARER_DATA, когда задана, побеждает
// xdg.DataHome целиком (appDirName к её значению не приписывается), и
// MediaDir/DBPath/SearchIndexPath оказываются внутри неё же. Использует тот
// же хелпер withTempDataHome (а не отдельный TestMain), потому что t.Setenv
// требует *testing.T — схема "сброс Once + t.Setenv" даёт изоляцию по тесту
// и автоуборку окружения.
func TestDataDirRespectsEnv(t *testing.T) {
	withTempDataHome(t)
	// Сбрасывать sync.Once повторно не нужно: withTempDataHome только что
	// это сделал, а DataDir() между тем и этим вызовом никто не звал.
	custom := t.TempDir()
	t.Setenv("VERSEBEARER_DATA", custom)

	dir, err := DataDir()
	if err != nil {
		t.Fatalf("DataDir: %v", err)
	}
	if dir != custom {
		t.Errorf("DataDir = %q, want %q (VERSEBEARER_DATA, no appDirName suffix)", dir, custom)
	}

	media, err := MediaDir()
	if err != nil {
		t.Fatalf("MediaDir: %v", err)
	}
	if want := filepath.Join(custom, "media"); media != want {
		t.Errorf("MediaDir = %q, want %q", media, want)
	}

	dbPath, err := DBPath()
	if err != nil {
		t.Fatalf("DBPath: %v", err)
	}
	if want := filepath.Join(custom, "versebearer.db"); dbPath != want {
		t.Errorf("DBPath = %q, want %q", dbPath, want)
	}

	idxPath, err := SearchIndexPath()
	if err != nil {
		t.Fatalf("SearchIndexPath: %v", err)
	}
	if want := filepath.Join(custom, "search.bleve"); idxPath != want {
		t.Errorf("SearchIndexPath = %q, want %q", idxPath, want)
	}
}
