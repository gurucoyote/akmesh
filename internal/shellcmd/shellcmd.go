// Package shellcmd is the command table of akmesh-shell: the shell and the
// package manager that runs its build scripts.
//
// Both need mvdan.cc/sh and its builtins - the shell to run a script, apk to run
// an install script in it - so they share one binary. A harness that never runs
// a script does not need it (docs/design/split.md).
package shellcmd

import (
	"io"
	"os"

	"akmesh/internal/apkbuild"
	"akmesh/internal/apkcmd"
	"akmesh/internal/apkguard"
	"akmesh/internal/apkreport"
	"akmesh/internal/cmdrun"
	"akmesh/internal/multicall"
	"akmesh/internal/toolsh"
	"akmesh/internal/usage"
)

// Prog is this binary: sh and gosh from the shell block, the four apk commands.
var Prog = multicall.Prog{
	Name: "akmesh-shell",
	Commands: map[string]cmdrun.Func{
		"sh":   toolsh.Commands["sh"],
		"gosh": toolsh.Commands["gosh"],
		"apk": func(args []string, stdin, stdout, stderr *os.File) int {
			return usage.RunNamedIO("apk", args, stdin, stdout, stderr,
				func(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
					return apkcmd.Run(cmd, args, stdin, stdout, stderr)
				})
		},
		"apkbuild": func(args []string, stdin, stdout, stderr *os.File) int {
			return usage.RunNamedIO("apkbuild", args, stdin, stdout, stderr,
				func(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
					return apkbuild.Run(args, stdout, stderr)
				})
		},
		"apkreport": func(args []string, stdin, stdout, stderr *os.File) int {
			return usage.RunNamedIO("apkreport", args, stdin, stdout, stderr,
				func(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
					return apkreport.Run(args, stdout, stderr)
				})
		},
		"apkguard": func(args []string, stdin, stdout, stderr *os.File) int {
			return usage.RunNamedIO("apkguard", args, stdin, stdout, stderr,
				func(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
					return apkguard.Run(args, stdout, stderr)
				})
		},
	},
	Others: []string{"akmesh", "akmesh-docs", "akmesh-write", "akmesh-net"},
}

// Main is the entry point of the binary, called from cmd/akmesh-shell.
func Main() int { return Prog.Main() }

// Run is the dispatch without the policy, so a test sees what a caller sees.
func Run(argv []string, stdin, stdout, stderr *os.File) int {
	return Prog.Run(argv, stdin, stdout, stderr)
}

// Names lists this binary's commands.
func Names() []string { return Prog.Names() }
