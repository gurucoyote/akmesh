//go:build !windows && !wasip1 && !js

package tools

import (
	"os"
	"strconv"
	"syscall"
)

func inodeOf(info os.FileInfo) string {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return strconv.FormatUint(uint64(st.Ino), 10)
	}
	return "-"
}

func wholeSystemSync() error {
	syscall.Sync()
	return nil
}
