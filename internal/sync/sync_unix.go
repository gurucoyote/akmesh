//go:build !windows && !wasip1 && !js

// Package sync implements sync, flushing the file system buffers.
package sync

import "syscall"

// flush is the platform half of Run: there is nothing to report on unix.
func flush() error {
	syscall.Sync()
	return nil
}
