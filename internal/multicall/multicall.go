// Package multicall is the dispatcher of the split binaries.
//
// The full akmesh binary is a gobusybox build: u-root's dispatcher answers and
// our commands hang off it. The split binaries (akmesh-docs, akmesh-write and
// later the rest) are plain Go programs, so they need their own dispatch - and
// exactly one implementation of it, or the four binaries drift apart in the way
// they answer a mistyped command (docs/design/split.md).
//
// Two ways in, for every one of them: the name the program was started as (a
// symlink called "rg"), or the first argument ("akmesh-docs rg"). A name the
// binary does not have is an error with the list of what it does have, never a
// silent success.
package multicall

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gurucoyote/akmesh/internal/cmdrun"
	"github.com/gurucoyote/akmesh/internal/policy"
)

// Prog is one dispatcher: its name, its table, and the other packages of the
// split so that the list can point at them.
type Prog struct {
	Name     string
	Commands map[string]cmdrun.Func
	Others   []string
}

// Names lists the commands in the order the help prints them.
func (p Prog) Names() []string {
	out := make([]string, 0, len(p.Commands))
	for name := range p.Commands {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// Main is the entry point of the binary: the policy first, then the dispatch.
func (p Prog) Main() int {
	// A rule that only holds in some binaries holds nowhere.
	policy.Enter()
	return p.Run(os.Args, os.Stdin, os.Stdout, os.Stderr)
}

// Run is the dispatch without the policy, so that a test exercises the same path
// as the binary.
func (p Prog) Run(argv []string, stdin, stdout, stderr *os.File) int {
	name := p.Name
	if len(argv) > 0 && argv[0] != "" {
		name = filepath.Base(argv[0])
	}
	var args []string
	if len(argv) > 1 {
		args = argv[1:]
	}
	if _, ok := p.Commands[name]; !ok {
		if len(args) == 0 {
			p.usage(name, stderr)
			return 2
		}
		if _, ok := p.Commands[args[0]]; ok {
			name, args = args[0], args[1:]
		} else {
			fmt.Fprintf(stderr, "%s: unknown command %q\n", name, args[0])
			p.usage(name, stderr)
			return 2
		}
	}
	// The command sees itself under its own name, the way u-root's dispatcher
	// presents it. httpd and other daemons detach by starting this binary again
	// and pass the name along from os.Args[0]; without this line the child would
	// start as "<binary> -f ..." and the dispatcher would read "-f" as a command
	// (measured 25.11.2026: httpd in akmesh-net printed a pid and never served).
	os.Args = append([]string{name}, args...)
	return cmdrun.RunArgs(p.Commands, name, args, stdin, stdout, stderr)
}

// usage names this binary's commands and the other packages. A command that is
// not here must not look as if it were, and the caller has to learn where it
// does live: that was the rule before the split and it stays the rule after it.
func (p Prog) usage(prog string, w io.Writer) {
	fmt.Fprintf(w, "usage: %s COMMAND [ARG...]\n", prog)
	fmt.Fprintf(w, "commands: %s\n", strings.Join(p.Names(), " "))
	if len(p.Others) > 0 {
		fmt.Fprintf(w, "other packages: %s\n", strings.Join(p.Others, " "))
	}
}
