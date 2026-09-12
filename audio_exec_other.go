//go:build !windows

package main

import "os/exec"

// hideWindow — нет консольного окна вне Windows, поэтому no-op.
func hideWindow(cmd *exec.Cmd) {}
