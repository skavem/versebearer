//go:build !windows

package main

// keepAwake — заглушка вне Windows: SetThreadExecutionState — Windows-only
// API, на macOS/Linux запрет гашения дисплея пока не реализован (проектор в
// проекте эксплуатируется на Windows). Интерфейс тот же, чтобы вызывающий
// код (db_outputs.go) собирался на всех платформах без build-тегов у себя.
func keepAwake() (release func()) {
	return func() {}
}
