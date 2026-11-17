// Command ext is the small companion to the u-root busybox in this repository.
//
// It carries the commands u-root does not build for Windows (or does not have
// at all), written against the Go standard library plus three small libraries:
// go-difflib and go-gitdiff for diff and patch, Go.Sed for sed and goawk for awk.
//
// Everything else is ours. The binary must stay small: no framework, no
// locale machinery, no shell. See docs/groesse.md for the budget.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// run is the signature every command implements. The dispatcher passes the
// arguments after the command name and the standard streams.
type run func(args []string, stdin *os.File, stdout, stderr *os.File) int

var commands = map[string]run{
	"awk":        runAwk,
	"basename":   runBasename,
	"cut":        runCut,
	"diff":       runDiff,
	"dirname":    runDirname,
	"hostname":   runHostname,
	"md5sum":     runSum("md5"),
	"nl":         runNl,
	"patch":      runPatch,
	"sed":        runSed,
	"sha1sum":    runSum("sha1"),
	"sha256sum":  runSum("sha256"),
	"sha512sum":  runSum("sha512"),
	"split":      runSplit,
	"stat":       runStat,
	"sync":       runSync,
	"tac":        runTac,
	"timeout":    runTimeout,
	"truncate":   runTruncate,
	"uname":      runUname,
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	name := os.Args[1]
	if name == "--list" || name == "-l" {
		list()
		return
	}
	// argv[0] dispatch: if the binary is called under another name, that name
	// is the command (symlinks work on Linux, copies on Windows).
	if base := filepath.Base(os.Args[0]); base != "ext" && base != "ext.exe" && commands[base] != nil {
		name = base
		os.Exit(commands[name](os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
	}
	fn, ok := commands[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "ext: unknown command %q\n", name)
		usage()
		os.Exit(2)
	}
	os.Exit(fn(os.Args[2:], os.Stdin, os.Stdout, os.Stderr))
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: ext <command> [args...]")
	fmt.Fprintln(os.Stderr, "       ext --list")
}

func list() {
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Println(name)
	}
}
