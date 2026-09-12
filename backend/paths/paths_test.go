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
