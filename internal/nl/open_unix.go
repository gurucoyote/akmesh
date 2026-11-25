//go:build !windows

package nl

import (
	"errors"
	"os"
)

// openFile opens a file for reading.
func openFile(name string) (*os.File, error) {
	return os.Open(name)
}

// sentence is the system's reason with glibc's capital letter, the way GNU
// prints it after the file name.
func sentence(err error) string {
	if pe, ok := errors.AsType[*os.PathError](err); ok {
		err = pe.Err
	}
	msg := err.Error()
	if msg != "" && msg[0] >= 'a' && msg[0] <= 'z' {
		msg = string(msg[0]-32) + msg[1:]
	}
	return msg
}
