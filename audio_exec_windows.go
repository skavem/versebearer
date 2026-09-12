//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideWindow подавляет мелькание консоли ffmpeg при импорте: без него
// на Windows на долю секунды выскакивало бы окно cmd.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
