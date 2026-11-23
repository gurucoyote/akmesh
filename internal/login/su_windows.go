//go:build windows

package login

import (
	"fmt"
	"io"
)

// RunSu reports that there is no su here: Windows has no setuid and no
// /etc/passwd to switch accounts from.
func RunSu(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "su: not supported on Windows (no setuid, accounts are managed by Windows)")
	return 1
}

// RunPasswd reports that passwords are not files here.
func RunPasswd(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fmt.Fprintln(stderr, "passwd: not supported on Windows (accounts are managed by Windows)")
	return 1
}
