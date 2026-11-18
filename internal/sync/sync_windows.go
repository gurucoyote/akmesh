//go:build windows

// Package sync implements sync, flushing the file system buffers.
package sync

import "errors"

// flush is the platform half of Run. Windows has no whole-system flush a
// process may call, so the command says so instead of pretending.
func flush() error {
	return errors.New("no whole-system sync on windows")
}
