package tools

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/forked/comm"
)

// runComm is the glue between the command table and the forked u-root command.
// The fork keeps the upstream source and its copyright; see
// internal/forked/comm/comm.go for the list of local changes.
func runComm(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return comm.Run(args, stdin, stdout, stderr)
}
