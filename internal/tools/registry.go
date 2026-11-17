// Package tools carries the commands that u-root either does not have or does
// not build for Windows: text tools, diff/patch, sed, awk, checksums and a few
// system queries.
//
// The implementation is one package with one table, so that the same code can
// be reached three ways: as a subcommand of the ext multicall binary, as a
// first-class command inside the combined busybox (one tiny main package per
// name), and from tests.
//
// Size matters here. See docs/groesse.md for the budget and the rules that keep
// this package small: standard library first, a small library only where the
// alternative would be a rewrite (Go.Sed, goawk, go-gitdiff).
package tools

import (
	"fmt"
	"os"
	"sort"
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
	"kill":      runKill,
	"killall":   runKillall,
	"md5sum":    runSum("md5"),
	"nl":        runNl,
	"patch":     runPatch,
	"ps":        runPS,
	"sed":       runSed,
	"sha1sum":   runSum("sha1"),
	"sha256sum": runSum("sha256"),
	"sha512sum": runSum("sha512"),
	"split":     runSplit,
	"stat":      runStat,
	"sync":      runSync,
	"tac":       runTac,
	"timeout":   runTimeout,
	"truncate":  runTruncate,
	"uname":     runUname,
}

// Run executes the named command with the arguments the program was started
// with, minus argv[0]. It is the entry point for the per-command main packages.
func Run(name string) int {
	fn, ok := Commands[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "ext: unknown command %q\n", name)
		return 2
	}
	var args []string
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}
	return fn(args, os.Stdin, os.Stdout, os.Stderr)
}

// Main is the entry point of the multicall binary: ext <command> [args...].
func Main(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	if args[0] == "--list" || args[0] == "-l" {
		List()
		return 0
	}
	fn, ok := Commands[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "ext: unknown command %q\n", args[0])
		usage()
		return 2
	}
	return fn(args[1:], os.Stdin, os.Stdout, os.Stderr)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: ext <command> [args...]")
	fmt.Fprintln(os.Stderr, "       ext --list")
}

// Names returns all command names in sorted order.
func Names() []string {
	names := make([]string, 0, len(Commands))
	for name := range Commands {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// List prints one command name per line.
func List() {
	for _, name := range Names() {
		fmt.Println(name)
	}
}
