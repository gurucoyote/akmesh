// Package cmdrun is the shared entry into a command table: the policy first,
// then the help gate, then the command's own parser.
//
// It exists so that a table need not live in one package. Each block of
// commands (files and text, documents, packages, network, shell) keeps its own
// table and its own Run; this package holds the three lines they all need. The
// linker then drops a block a binary does not name - which was impossible while
// every command sat in one map inside internal/tools (acmesh-58me, measured in
// docs/design/split.md).
package cmdrun

import (
	"fmt"
	"os"
	"path/filepath"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

// Func is the signature every command implements: the arguments after the
// command name plus the standard streams, and the exit code.
type Func = usage.Func

// Run executes the named command with the arguments the program was started
// with, minus argv[0]. The policy is applied here, so a block's Run is as safe
// as the per-command mains that call policy.Enter themselves.
func Run(table map[string]Func, name string) int {
	// A broken policy stops the process: running unconfined after a typo would
	// be the worst failure mode this tool kit could have.
	policy.Enter()
	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}
	return RunArgs(table, name, args, os.Stdin, os.Stdout, os.Stderr)
}

// RunArgs executes one command the way the dispatcher does: the shared --help
// answer first, then the command's own parser. Tests call it so that they
// exercise the same path as the binary (acmesh-py8).
//
// The help is answered here and not inside each command, so a broken option
// parser cannot swallow "--help" as an operand: before this, hostname read it
// as a new host name and sync as a file to flush.
func RunArgs(table map[string]Func, name string, args []string, stdin, stdout, stderr *os.File) int {
	fn, ok := table[name]
	if !ok {
		// Only reachable through a table that disagrees with cmd/<name>; the
		// message names the program that was actually started.
		fmt.Fprintf(os.Stderr, "%s: unknown command %q\n", ProgramName(), name)
		return 2
	}
	return usage.Run(name, args, stdin, stdout, stderr, fn)
}

// ProgramName is the name the program was invoked as, so a message never claims
// to come from a binary that does not exist.
func ProgramName() string {
	if len(os.Args) == 0 || os.Args[0] == "" {
		return "akmesh"
	}
	return filepath.Base(os.Args[0])
}
