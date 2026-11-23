// Package logger is the thin command wrapper around internal/syslog: one package
// per command, as the tool table expects, one implementation for the trio.
package logger

import (
	"io"

	"akmesh/internal/syslog"
)

// Run is the command entry point; stdin is read when no message is given.
func Run(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return syslog.RunLogger(name, args, stdin, stdout, stderr)
}
