package tools

import (
	"os"

	"acmesh/internal/forked/wc"
)

// runWc is the glue between the command table and the forked u-root command.
// The fork keeps the upstream source and its copyright; see
// internal/forked/wc/wc.go for the list of local changes.
func runWc(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return wc.Run(args, stdin, stdout, stderr)
}
