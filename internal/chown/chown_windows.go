//go:build windows

// Package chown changes the owner and the group of files. Windows has no POSIX
// owner: access is granted through ACLs, which icacls edits. Saying that is
// better than printing a success that changed nothing (acmesh-6p8).
package chown

import (
	"fmt"
	"io"
)

// Run reports that chown has no meaning here.
func Run(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "chown: not supported on Windows (no POSIX ownership; access is controlled by ACLs)")
	return 1
}
