// Package tooldiff carries diff and patch.
//
// Why its own package: both share go-gitdiff, and a binary that only reads a
// file should not carry a unified-diff engine (acmesh-rnyu).
package tooldiff

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/cmdrun"
	"github.com/gurucoyote/akmesh/internal/diffpatch"
)

// Commands is this block's table.
var Commands = map[string]cmdrun.Func{
	"diff":  runDiff,
	"patch": runPatch,
}

// Run is the entry point for cmd/<name>/main.go.
func Run(name string) int { return cmdrun.Run(Commands, name) }

func runDiff(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return diffpatch.Run("diff", args, stdin, stdout, stderr)
}

func runPatch(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return diffpatch.Run("patch", args, stdin, stdout, stderr)
}
