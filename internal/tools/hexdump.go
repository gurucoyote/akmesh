package tools

import (
	"os"

	"akmesh/internal/forked/hexdump"
)

// runHexdump is the glue between the command table and the forked u-root
// command. The fork keeps the upstream source and its copyright; see
// internal/forked/hexdump/hexdump.go for the list of local changes.
func runHexdump(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return hexdump.Run(args, stdin, stdout, stderr)
}
