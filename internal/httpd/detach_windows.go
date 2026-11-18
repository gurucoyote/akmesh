//go:build windows

package httpd

import "syscall"

// detachedAttr gives the background server a process without its own console.
func detachedAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: 0x00000008} // DETACHED_PROCESS
}
