package tools

import (
	"os"

	"akmesh/internal/forked/head"
)

// runHead is the glue between the command table and the forked u-root command.
// The fork keeps the upstream code and its copyright; see
// internal/forked/head/head.go for the list of local changes.
func runHead(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return head.Run(args, stdin, stdout, stderr)
}
