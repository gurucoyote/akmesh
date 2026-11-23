package tools

import (
	"os"

	"akmesh/internal/forked/uniq"
)

// runUniq is the glue between the command table and the forked u-root command.
// The fork keeps the upstream source and its copyright; see
// internal/forked/uniq/uniq.go for the list of local changes.
func runUniq(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return uniq.Run(args, stdin, stdout, stderr)
}
