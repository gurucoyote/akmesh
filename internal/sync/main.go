// Package sync implements sync: flush the file system buffers, or, with file
// operands, make sure the written data of those files reaches the device.
package sync

import (
	"fmt"
	"os"
)

// Run implements sync [FILE...]. A whole-system sync does not exist on Windows;
// there we say so instead of pretending.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	if len(args) == 0 {
		if flush() != nil {
			fmt.Fprintln(stderr, "sync: not supported on this platform: pass FILE operands to sync specific files")
			return 1
		}
		return 0
	}
	for _, name := range args {
		f, err := os.OpenFile(name, os.O_RDWR, 0o644)
		if err != nil {
			fmt.Fprintf(stderr, "sync: %v\n", err)
			return 1
		}
		err = f.Sync()
		f.Close()
		if err != nil {
			fmt.Fprintf(stderr, "sync: %v\n", err)
			return 1
		}
	}
	return 0
}
