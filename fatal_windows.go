//go:build windows

package main

import (
	"log"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Флаги MessageBoxW. golang.org/x/sys/windows их не экспортирует (тот же
// класс, что и с SetThreadExecutionState в wake_windows.go) — грузим
// user32.dll вручную и объявляем константы локально.
const (
	mbOK            = 0x00000000
	mbIconError     = 0x00000010
	mbSystemModal   = 0x00001000
	mbSetForeground = 0x00010000 // без него окно без права на передний план получает только мигающую кнопку в панели задач
)

var procMessageBoxW = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")

// fatalDialog показывает оператору причину, по которой программа не
// запустилась. В GUI-сборке на Windows (-H=windowsgui) консоли нет: log и
// panic уходят в пустоту, поэтому нужен нативный MessageBox. Текст
// дублируется в log.Println — при запуске из консоли он пригодится.
// runtime.LockOSThread не нужен: в отличие от SetThreadExecutionState,
// MessageBoxW не привязан к вызывающему потоку.
func fatalDialog(title, text string) {
	log.Println(title+":", text)

	// LazyProc.Call паникует, если сама процедура не нашлась (Find() внутри
	// Call не проверяется) — обработчик фатальной ошибки не должен падать
	// ещё раз с паникой поверх исходного отказа. На отсутствие MessageBoxW
	// в user32.dll в реальности рассчитывать не приходится, но лог выше уже
	// не даст пропасть причине бесследно.
	if err := procMessageBoxW.Find(); err != nil {
		return
	}

	// UTF16PtrFromString отказывается конвертировать строку со встроенным
	// NUL. Раньше это тихо съедало единственный видимый оператору канал
	// (return без окна) — в GUI-сборке (-H=windowsgui) лог никто не
	// увидит. NUL в тексте ошибки не ожидается, но вырезать его дешевле,
	// чем гадать.
	title = strings.ReplaceAll(title, "\x00", "")
	text = strings.ReplaceAll(text, "\x00", "")

	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	textPtr, err := windows.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(mbOK|mbIconError|mbSystemModal|mbSetForeground),
	)
}
