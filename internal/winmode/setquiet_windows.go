//go:build windows

package winmode

import (
	"errors"
	"io/fs"

	"golang.org/x/sys/windows"
)

// SetQuiet records mode in the file's ACL and stays silent where the filesystem
// cannot hold one (a WSL share, a FAT stick): chmod already did all the platform
// offers there, and failing would break a script that chmods on a share
// (acmesh-m6x). Every other error is reported.
func SetQuiet(path string, mode fs.FileMode) error {
	err := Set(path, mode)
	if errors.Is(err, windows.ERROR_INVALID_FUNCTION) || errors.Is(err, windows.ERROR_NOT_SUPPORTED) {
		return nil
	}
	return err
}
