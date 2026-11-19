package tools

import (
	"os"

	"akmesh/internal/forked/find"
)

// runFind is the glue between the command table and the forked u-root command.
// The fork keeps the upstream source and its copyright; see
// internal/forked/find/find.go for the list of local changes.
func runFind(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return find.Run(args, stdin, stdout, stderr)
}
