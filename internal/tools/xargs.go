package tools

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/forked/xargs"
)

// runXargs is the glue between the command table and the forked u-root command.
// The fork keeps the upstream source and its copyright; see
// internal/forked/xargs/xargs.go for the list of local changes.
func runXargs(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return xargs.Run(args, stdin, stdout, stderr)
}
