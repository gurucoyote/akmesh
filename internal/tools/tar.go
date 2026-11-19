package tools

import (
	"os"

	"akmesh/internal/forked/tar"
)

// runTar is the glue between the command table and the forked u-root command.
// The fork keeps the upstream source and its copyright; see
// internal/forked/tar/tar.go for the list of local changes.
func runTar(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return tar.Run(args, stdin, stdout, stderr)
}
