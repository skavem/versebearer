//go:build !windows

package main

import "log"

// fatalDialog вне Windows — нативного окна нет, обходимся логом в stderr.
func fatalDialog(title, text string) {
	log.Println(title+":", text)
}
