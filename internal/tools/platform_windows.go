//go:build windows

package tools

import (
	"errors"
	"os"
)

func inodeOf(info os.FileInfo) string { return "-" }

func wholeSystemSync() error {
	return errors.New("no whole-system sync on windows")
}
