//go:build windows

package nl

import (
	"errors"
	"os"
)

// openFile opens a file for reading.
func openFile(name string) (*os.File, error) {
	return os.Open(name)
}

// sentence is the system's reason, capitalised when the system writes it in
// lower case.
func sentence(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	msg := err.Error()
	if msg != "" && msg[0] >= 'a' && msg[0] <= 'z' {
		msg = string(msg[0]-32) + msg[1:]
	}
	return msg
}
