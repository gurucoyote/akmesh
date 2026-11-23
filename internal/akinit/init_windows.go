//go:build windows

// Package akinit has no Windows implementation: there is no PID 1, no SIGCHLD
// and no orphan adoption to do. Windows services belong to the SCM, and this
// project does not register host services (AGENTS.md, "Windows: no traces in
// the host").
package akinit

import (
	"fmt"
	"io"
)

// Run says so and fails, instead of pretending to be an init.
func Run(name string, args []string, stdout, stderr io.Writer) int {
	fmt.Fprintf(stderr, "%s: not supported on Windows (no PID 1; services belong to the SCM)\n", name)
	return 1
}
