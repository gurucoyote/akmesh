// Package tooltar carries tar.
//
// Why its own package: the forked u-root tar is a large feature set, and a
// binary that only copies or lists files should not pay for it - measured in
// docs/design/split.md (acmesh-rnyu).
package tooltar

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/cmdrun"
	"github.com/gurucoyote/akmesh/internal/forked/tar"
)

// Commands is this block's table.
var Commands = map[string]cmdrun.Func{
	"tar": runTar,
}

// Run is the entry point for cmd/tar/main.go.
func Run(name string) int { return cmdrun.Run(Commands, name) }

func runTar(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return tar.Run(args, stdin, stdout, stderr)
}
