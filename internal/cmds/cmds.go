// Package cmds is the full command table: the blocks joined into one map.
//
// It exists for tests and for anything that must know every command, never for a
// cmd/<name>/main.go: importing it there would pull every block - and with them
// every library - back into that binary, which is exactly what the block split
// removed (acmesh-58me, docs/design/split.md).
package cmds

import (
	"fmt"
	"os"

	"github.com/gurucoyote/akmesh/internal/cmdrun"
	"github.com/gurucoyote/akmesh/internal/tooldiff"
	"github.com/gurucoyote/akmesh/internal/tooljq"
	"github.com/gurucoyote/akmesh/internal/toolnet"
	"github.com/gurucoyote/akmesh/internal/toolpkg"
	"github.com/gurucoyote/akmesh/internal/tools"
	"github.com/gurucoyote/akmesh/internal/toolsed"
	"github.com/gurucoyote/akmesh/internal/toolsh"
	"github.com/gurucoyote/akmesh/internal/toolsvc"
	"github.com/gurucoyote/akmesh/internal/tooltar"
)

// All is every command the full binary carries, keyed by name.
var All = merge(tools.Commands, toolsed.Commands, tooldiff.Commands, toolpkg.Commands, toolnet.Commands, toolsh.Commands, toolsvc.Commands, tooljq.Commands, tooltar.Commands)

// merge joins the block tables and refuses a name that two blocks claim: two
// entries would mean one of them is dead code that looks alive.
func merge(tables ...map[string]cmdrun.Func) map[string]cmdrun.Func {
	all := map[string]cmdrun.Func{}
	for _, t := range tables {
		for name, fn := range t {
			if _, dup := all[name]; dup {
				panic(fmt.Sprintf("command %q is in two blocks", name))
			}
			all[name] = fn
		}
	}
	return all
}

// Run executes the named command with this process's arguments, the way a
// dispatcher would.
func Run(name string) int { return cmdrun.Run(All, name) }

// RunArgs executes one command with the given arguments and streams.
func RunArgs(name string, args []string, stdin, stdout, stderr *os.File) int {
	return cmdrun.RunArgs(All, name, args, stdin, stdout, stderr)
}
