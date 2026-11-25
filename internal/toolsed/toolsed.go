// Package toolsed carries sed and awk.
//
// Why its own package: the two pull Go.Sed and goawk, which together are the
// largest foreign chunk in the base binary and are useless to every command
// that only copies, lists or searches files (acmesh-rnyu, measured in
// docs/design/split.md).
package toolsed

import (
	"os"

	"akmesh/internal/cmdrun"
	"akmesh/internal/sedawk"
)

// Commands is this block's table.
var Commands = map[string]cmdrun.Func{
	"awk": runAwk,
	"sed": runSed,
}

// Run is the entry point for cmd/<name>/main.go.
func Run(name string) int { return cmdrun.Run(Commands, name) }

func runSed(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return sedawk.Run("sed", args, stdin, stdout, stderr)
}

func runAwk(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return sedawk.Run("awk", args, stdin, stdout, stderr)
}
