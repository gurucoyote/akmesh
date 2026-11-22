//go:build windows

// Package login answers on Windows with the truth: there is no setuid and no
// /etc/passwd, so a session as another user is not a shell feature here.
package login

import (
	"fmt"
	"io"
)

// Run is the command entry point.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Fprintln(stdout, "usage: login [-f] USER")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "login is not supported on Windows: there is no /etc/passwd")
			fmt.Fprintln(stdout, "and no setuid. Use the Windows account you started the shell as.")
			return 0
		}
	}
	fmt.Fprintln(stderr, "login: not supported on Windows")
	return 1
}
