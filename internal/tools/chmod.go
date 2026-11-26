package tools

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/forked/chmod"
)

// runChmod is the glue between the command table and the forked u-root command.
// The fork keeps the upstream source and its copyright; see
// internal/forked/chmod/chmod.go for the list of local changes.
func runChmod(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return chmod.Run(args, stdin, stdout, stderr)
}
