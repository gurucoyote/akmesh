//go:build windows

// Local addition, no upstream source: chmod schreibt den Modus in die DACL (acmesh-m6x).

package chmod

import (
	"io/fs"

	"akmesh/internal/winmode"
)

// recordMode writes the mode into the file's ACL, because os.Chmod on Windows
// only toggles the read-only attribute and a chmod 755 changed nothing
// (acmesh-m6x). SetQuiet keeps the one documented exception - a filesystem
// without DACLs - in one place, shared with mkcert.
func recordMode(path string, mode fs.FileMode) error {
	return winmode.SetQuiet(path, mode)
}
