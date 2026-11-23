//go:build windows

// Package getty has no Windows implementation: there is no console device to own
// and no login to start. Windows asks for a password in its own session manager,
// and this project does not touch it (AGENTS.md, "Windows: no traces in the
// host").
package getty

import (
	"fmt"
	"io"
)

// Run says so and fails.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintln(stdout, "usage: getty [-f ISSUE] [-t TTY]")
			fmt.Fprintln(stdout, "not supported on Windows: there is no console login to own")
			return 0
		}
	}
	fmt.Fprintln(stderr, "getty: not supported on Windows (no console login to own)")
	return 1
}
