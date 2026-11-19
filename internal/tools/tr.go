package tools

import (
	"os"

	"akmesh/internal/forked/tr"
)

// runTr is the glue between the command table and the forked u-root command.
// The fork keeps the upstream source and its copyright; see
// internal/forked/tr/tr.go for the list of local changes.
func runTr(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return tr.Run(args, stdin, stdout, stderr)
}
