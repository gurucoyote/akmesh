package tools

import (
	"os"

	"acmesh/internal/forked/base64"
)

// runBase64 is the glue between the command table and the forked u-root command.
// The fork keeps the upstream source and its copyright; see
// internal/forked/base64/base64.go for the list of local changes.
func runBase64(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return base64.Run(args, stdin, stdout, stderr)
}
