// Package tools carries the commands that u-root either does not have or does
// not build for Windows: text tools, diff/patch, sed, awk, checksums and a few
// system queries.
//
// The implementation is one package with one table, so that the same code is
// reached two ways: as a command of the acmesh binary (one tiny main package
// per name, which calls Run) and from tests.
//
// Size matters here. See docs/groesse.md for the budget and the rules that keep
// this package small: standard library first, a small library only where the
// alternative would be a rewrite (Go.Sed, goawk, go-gitdiff).
package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"acmesh/internal/policy"
	"acmesh/internal/shuf"
	"acmesh/internal/sort"
)

// Func is the signature every command implements. It receives the arguments
// after the command name plus the standard streams, and returns the exit code.
type Func func(args []string, stdin *os.File, stdout, stderr *os.File) int

// Commands maps a command name to its implementation.
var Commands = map[string]Func{
	"awk":       runAwk,
	"basename":  runBasename,
	"cut":       runCut,
	"diff":      runDiff,
	"dirname":   runDirname,
	"env":       runEnv,
	"df":        runDf,
	"file":      runFile,
	"hostname":  runHostname,
	"du":        runDu,
	"free":      runFree,
	"iconv":     runIconv,
	"jq":        runJq,
	"kill":      runKill,
	"killall":   runKillall,
	"md5sum":    runSum("md5"),
	"nl":        runNl,
	"patch":     runPatch,
	"policy":    runPolicy,
	"ps":        runPS,
	"sed":       runSed,
	"sha1sum":   runSum("sha1"),
	"sha256sum": runSum("sha256"),
	"sha512sum": runSum("sha512"),
	"sh":        runSh,
	"shuf":      shufcmd.Run,
	"sort":      sortcmd.Run,
	"split":     runSplit,
	"stat":      runStat,
	"sync":      runSync,
	"tac":       runTac,
	"timeout":   runTimeout,
	"truncate":  runTruncate,
	"uname":     runUname,
	"gosh":      runSh,
}

// policyOnce makes sure the policy is applied exactly once per process, before
// any command runs. A broken policy stops the process: running unconfined after a
// typo would be the worst failure mode this binary could have.
var policyOnce sync.Once

func ensurePolicy() {
	policyOnce.Do(func() {
		if err := policy.InitFromEnv(); err != nil {
			fmt.Fprintf(os.Stderr, "acmesh: %v\n", err)
			os.Exit(1)
		}
	})
}

// Run executes the named command with the arguments the program was started
// with, minus argv[0]. It is the entry point for the per-command main packages.
func Run(name string) int {
	ensurePolicy()
	fn, ok := Commands[name]
	if !ok {
		// Only reachable through a table that disagrees with cmd/<name>; the
		// message names the program that was actually started.
		fmt.Fprintf(os.Stderr, "%s: unknown command %q\n", programName(), name)
		return 2
	}
	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}
	return fn(args, os.Stdin, os.Stdout, os.Stderr)
}

// programName is the name the program was invoked as, so a message never claims
// to come from a binary that does not exist.
func programName() string {
	if len(os.Args) == 0 || os.Args[0] == "" {
		return "acmesh"
	}
	return filepath.Base(os.Args[0])
}
