//go:build !windows

// Local addition, no upstream source: auf anderen Zielen ist chmod(2) der Schreiber.

package chmod

import "io/fs"

// recordMode is a no-op here: chmod(2) is the writer winmode stands in for on
// Windows, so there is nothing to record (acmesh-m6x).
func recordMode(string, fs.FileMode) error { return nil }
