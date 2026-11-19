package tools

import (
	"os"

	"akmesh/internal/forked/cp"
)

// runCp is the glue between the command table and the forked u-root command.
// The fork keeps the upstream source and its copyright; see
// internal/forked/cp/cp.go for the list of local changes.
func runCp(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return cp.Run(args, stdin, stdout, stderr)
}
