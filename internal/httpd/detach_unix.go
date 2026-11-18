//go:build !windows

package httpd

import "syscall"

// detachedAttr puts the background server in its own session, so that closing
// the terminal that started it does not take it down.
func detachedAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
