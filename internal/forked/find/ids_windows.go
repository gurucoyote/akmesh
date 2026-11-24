//go:build windows

// Local addition, no upstream source: uid/gid names for find on Windows.

package find

import "os"

// idOf has no owner on Windows; -user and -group cannot match.
func idOf(os.FileInfo, bool) (uint32, bool) { return 0, false }

// devOf has no device number here, so -xdev has no effect on Windows.
func devOf(os.FileInfo) (uint64, bool) { return 0, false }

// idsAvailable is false: there is no uid or gid to look up.
func idsAvailable() bool { return false }
