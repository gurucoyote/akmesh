package tools

import (
	"os"

	"acmesh/internal/forked/touch"
)

// runTouch is the glue between the command table and the forked u-root command.
// The fork keeps the upstream source and its copyright; see
// internal/forked/touch/touch.go for the list of local changes.
func runTouch(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return touch.Run(args, stdin, stdout, stderr)
}
