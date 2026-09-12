//go:build windows

package main

import (
	"runtime"
	"sync"

	"golang.org/x/sys/windows"
)

// Коды состояния выполнения потока для SetThreadExecutionState.
// golang.org/x/sys/windows их не экспортирует (ни функции, ни констант —
// проверено), поэтому DLL подгружается вручную через kernel32.dll.
const (
	esContinuous      = 0x80000000
	esSystemRequired  = 0x00000001
	esDisplayRequired = 0x00000002
)

var procSetThreadExecutionState = windows.NewLazySystemDLL("kernel32.dll").NewProc("SetThreadExecutionState")

// keepAwake запрещает системе гасить дисплеи, пока открыто окно вывода.
// Флаг ES_CONTINUOUS — ПОТОКОВЫЙ: выставленный из обычной горутины, он
// слетит молча, как только планировщик Go переедет на другой поток (у Go
// нет привязки горутины к ОС-потоку без LockOSThread). Поэтому — своя
// горутина с runtime.LockOSThread, которая держит флаг выставленным, пока
// не придёт сигнал через stop, и только тогда снимает его тем же потоком.
func keepAwake() (release func()) {
	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)

		procSetThreadExecutionState.Call(uintptr(esContinuous | esSystemRequired | esDisplayRequired))
		<-stop
		// Снимаем флаг тем же потоком, что его выставил — иначе значение
		// молча не применится (флаг привязан к вызывающему потоку).
		procSetThreadExecutionState.Call(uintptr(esContinuous))
	}()

	var once sync.Once
	return func() {
		once.Do(func() {
			close(stop)
			<-done
		})
	}
}
