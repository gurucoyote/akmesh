//go:build !linux

package mount

import (
	"fmt"
	"os"
)

// Run on a platform without mount(2): the command is Linux-only, and saying so
// is better than a name that silently does nothing.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	fmt.Fprintln(stderr, "mount: not available on this platform")
	return 1
}
