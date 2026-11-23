// Package syslogd is the thin command wrapper around internal/syslog: one package
// per command, as the tool table expects, one implementation for the trio.
package syslogd

import (
	"io"

	"akmesh/internal/syslog"
)

// Run is the command entry point.
func Run(name string, args []string, stdout, stderr io.Writer) int {
	return syslog.RunSyslogd(name, args, stdout, stderr)
}
