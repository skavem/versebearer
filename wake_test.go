package main

import "testing"

// TestOutputWindowWake_CountsTransitionsOnly проверяет счётчик открытых окон
// вывода без реальных окон/устройства: keepAwakeFn подменяется фейком, как
// openDevice в audio_player.go. Проверяется именно логика переходов
// (0→1 зовёт keepAwake, 1→0 зовёт release), а не сам keepAwake
// (wake_windows.go/wake_other.go), который трогает системный API.
func TestOutputWindowWake_CountsTransitionsOnly(t *testing.T) {
	g := &DbHandler{}
	var keepAwakeCalls, releaseCalls int
	g.keepAwakeFn = func() func() {
		keepAwakeCalls++
		return func() { releaseCalls++ }
	}

	g.registerOutputWindowOpened()
	if keepAwakeCalls != 1 {
		t.Fatalf("first open: want 1 keepAwake call, got %d", keepAwakeCalls)
	}

	g.registerOutputWindowOpened()
	g.registerOutputWindowOpened()
	if keepAwakeCalls != 1 {
		t.Fatalf("further opens must not re-call keepAwake, got %d calls", keepAwakeCalls)
	}

	g.registerOutputWindowClosed()
	g.registerOutputWindowClosed()
	if releaseCalls != 0 {
		t.Fatalf("closing down to 1 remaining open window must not release yet, got %d calls", releaseCalls)
	}

	g.registerOutputWindowClosed()
	if releaseCalls != 1 {
		t.Fatalf("closing the last window: want 1 release call, got %d", releaseCalls)
	}

	// Лишнее закрытие сверх нуля не должно уйти в минус и повторно звать release.
	g.registerOutputWindowClosed()
	if releaseCalls != 1 {
		t.Fatalf("extra close beyond zero must not call release again, got %d", releaseCalls)
	}

	g.registerOutputWindowOpened()
	if keepAwakeCalls != 2 {
		t.Fatalf("reopening after reaching zero: want 2nd keepAwake call, got %d", keepAwakeCalls)
	}
}

// TestKeepAwake_ReleaseIsIdempotent проверяет, что реальный keepAwake (со
// своей горутиной на LockOSThread) не виснет и не паникует при повторном
// release() — main.go может закрыть окно вывода, чей release() уже был
// вызван раньше при закрытии последнего окна.
func TestKeepAwake_ReleaseIsIdempotent(t *testing.T) {
	release := keepAwake()
	release()
	release()
}
