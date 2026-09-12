package main

import "testing"

// TestOutputWindowWake_CountsTransitionsOnly проверяет переходы keepAwake по
// множеству РАЗНЫХ окон вывода без реальных окон/устройства: keepAwakeFn
// подменяется фейком, как openDevice в audio_player.go. Проверяется именно
// логика переходов (пусто->непусто зовёт keepAwake, непусто->пусто зовёт
// release), а не сам keepAwake (wake_windows.go/wake_other.go), который
// трогает системный API.
func TestOutputWindowWake_CountsTransitionsOnly(t *testing.T) {
	g := &DbHandler{}
	var keepAwakeCalls, releaseCalls int
	g.keepAwakeFn = func() func() {
		keepAwakeCalls++
		return func() { releaseCalls++ }
	}

	g.registerOutputWindowOpened("out-1")
	if keepAwakeCalls != 1 {
		t.Fatalf("first open: want 1 keepAwake call, got %d", keepAwakeCalls)
	}

	g.registerOutputWindowOpened("out-2")
	g.registerOutputWindowOpened("out-3")
	if keepAwakeCalls != 1 {
		t.Fatalf("further opens must not re-call keepAwake, got %d calls", keepAwakeCalls)
	}

	g.registerOutputWindowClosed("out-1")
	g.registerOutputWindowClosed("out-2")
	if releaseCalls != 0 {
		t.Fatalf("closing down to 1 remaining open window must not release yet, got %d calls", releaseCalls)
	}

	g.registerOutputWindowClosed("out-3")
	if releaseCalls != 1 {
		t.Fatalf("closing the last window: want 1 release call, got %d", releaseCalls)
	}

	// Лишнее закрытие уже закрытого/неизвестного имени не должно повторно
	// звать release.
	g.registerOutputWindowClosed("out-3")
	if releaseCalls != 1 {
		t.Fatalf("extra close beyond empty set must not call release again, got %d", releaseCalls)
	}

	g.registerOutputWindowOpened("out-1")
	if keepAwakeCalls != 2 {
		t.Fatalf("reopening after reaching empty: want 2nd keepAwake call, got %d", keepAwakeCalls)
	}
}

// TestOutputWindowWake_DuplicateNameDoesNotLeak — ревью (LOW): двойной
// StartOutput для ОДНОГО И ТОГО ЖЕ вывода (двойной клик, повторный вызов до
// того как предыдущее окно с тем же именем успело закрыться) раньше считал
// СЧЁТЧИКОМ ВЫЗОВОВ open, а не реально открытых окон — второй вызов поднимал
// счётчик до 2, но реально закроется (пришлёт WindowClosing) только ОДНО
// окно с этим именем: счётчик застревал бы на 1 навсегда, и дисплеи не гасли
// бы до перезапуска приложения. Множество, ключом по имени окна, схлопывает
// повторную регистрацию вместо того чтобы копить её.
func TestOutputWindowWake_DuplicateNameDoesNotLeak(t *testing.T) {
	g := &DbHandler{}
	var releaseCalls int
	g.keepAwakeFn = func() func() {
		return func() { releaseCalls++ }
	}

	g.registerOutputWindowOpened("out-1")
	g.registerOutputWindowOpened("out-1") // тот же вывод, повторный StartOutput

	g.registerOutputWindowClosed("out-1") // закрылось единственное реальное окно
	if releaseCalls != 1 {
		t.Fatalf("closing the only real window for a duplicated name must release immediately, got %d calls", releaseCalls)
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
