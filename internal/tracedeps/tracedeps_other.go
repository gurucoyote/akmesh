//go:build !linux || !amd64

// Command tracedeps needs ptrace and the amd64 register layout, so it exists
// there only. On any other target it says so instead of pretending
// (acmesh-n0p).
package tracedeps

import (
	"fmt"
	"os"
)

// Run reports that this target cannot trace.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	fmt.Fprintln(stderr, "tracedeps: tracing is built for linux/amd64 only")
	return 2
}
