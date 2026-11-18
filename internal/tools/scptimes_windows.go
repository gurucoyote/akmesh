//go:build windows

package tools

import (
	"os"
	"time"
)

// fileTimes carries the access time where the platform provides one.
type fileTimes struct{ atime time.Time }

// statTimes has nothing to report on Windows: the file information that Go reads
// contains the modification time only, and asking for more would need a handle
// per file during a bulk copy.
func statTimes(info os.FileInfo) (fileTimes, bool) { return fileTimes{}, false }
