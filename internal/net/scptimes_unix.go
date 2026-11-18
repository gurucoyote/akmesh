//go:build linux

package net

import (
	"os"
	"syscall"
	"time"
)

// fileTimes carries the access time, which Go's FileInfo does not expose.
type fileTimes struct{ atime time.Time }

// statTimes reads the access time from the raw stat structure.
func statTimes(info os.FileInfo) (fileTimes, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileTimes{}, false
	}
	return fileTimes{atime: time.Unix(st.Atim.Sec, st.Atim.Nsec)}, true
}
