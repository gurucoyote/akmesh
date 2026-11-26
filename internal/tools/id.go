package tools

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/forked/id"
)

// runID is the glue between the command table and the forked u-root id.
// The fork keeps the upstream source and its copyright; see
// internal/forked/id/id.go for the list of local changes.
func runID(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return id.Run(args, stdin, stdout, stderr)
}
