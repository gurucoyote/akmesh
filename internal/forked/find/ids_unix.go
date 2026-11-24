//go:build !windows

// Local addition, no upstream source: uid/gid names for find on unix.

package find

import (
	"os"
	"syscall"
)

// idOf returns the owner or group of a file.
func idOf(info os.FileInfo, isUser bool) (uint32, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	if isUser {
		return st.Uid, true
	}
	return st.Gid, true
}

// devOf returns the device a file lives on, for -xdev.
func devOf(info os.FileInfo) (uint64, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Dev), true
}

// idsAvailable reports whether user and group lookups can work at all.
func idsAvailable() bool { return true }
