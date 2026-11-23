//go:build windows

// Package setsid starts a program in its own session. Windows has no sessions
// in the POSIX sense: a process there does not die with the console it was
// started from.
package setsid

import (
	"fmt"
	"os"
)

// Run explains that setsid has nothing to do here instead of pretending.
func Run(cmd string, args []string, stdin *os.File, stdout, stderr *os.File) int {
	fmt.Fprintln(stderr, "setsid: not supported on Windows (a Windows process does not die with its console)")
	return 1
}
