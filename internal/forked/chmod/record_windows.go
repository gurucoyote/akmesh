//go:build windows

// Local addition, no upstream source: chmod schreibt den Modus in die DACL (acmesh-m6x).

package chmod

import (
	"errors"
	"io/fs"

	"golang.org/x/sys/windows"

	"akmesh/internal/winmode"
)

// recordMode writes the mode into the file's ACL, because os.Chmod on Windows
// only toggles the read-only attribute and a chmod 755 changed nothing
// (acmesh-m6x).
func recordMode(path string, mode fs.FileMode) error {
	err := winmode.Set(path, mode)
	// A filesystem without DACLs - the WSL share, a FAT stick - cannot record a
	// mode at all. os.Chmod has done everything the platform offers there, and
	// failing would break a script that chmods on a share, so this stays quiet;
	// stat then keeps reporting the platform's own 0666, so nothing claims a
	// mode that is not there.
	if errors.Is(err, windows.ERROR_INVALID_FUNCTION) || errors.Is(err, windows.ERROR_NOT_SUPPORTED) {
		return nil
	}
	return err
}
