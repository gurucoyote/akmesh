// Package toolsh carries the shell entry point: sh, and gosh under the name the
// old command list used.
//
// It is its own package because the shell brings mvdan.cc/sh and its builtins
// with it, which no other block needs - except the package block, which runs
// scripts through it.
package toolsh

import (
	"os"
	"path/filepath"

	"github.com/gurucoyote/akmesh/internal/cmdrun"
	"github.com/gurucoyote/akmesh/internal/sh"
)

// Commands is the shell block's table.
var Commands = map[string]cmdrun.Func{
	"sh":   runSh,
	"gosh": runSh,
}

// Run is the entry point for cmd/<name>/main.go.
func Run(name string) int { return cmdrun.Run(Commands, name) }

func runSh(args []string, stdin *os.File, stdout, stderr *os.File) int {
	// The shell answers $0 with the name it was called with, so the dispatcher's
	// binary path must not leak into the session (acmesh-51b). "gosh" stays
	// reachable under the name the old command list carried.
	name := "sh"
	if base := filepath.Base(os.Args[0]); base == "gosh" {
		name = "gosh"
	}
	return sh.RunAs(name, args, stdin, stdout, stderr)
}
