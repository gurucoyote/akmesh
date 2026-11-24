//go:build !windows

package apkscript

import (
	"syscall"
)

// mknodNull creates a character device 1:3, the classical /dev/null.
func mknodNull(path string) error {
	return syscall.Mknod(path, syscall.S_IFCHR|0o666, int((1<<8)|3))
}
