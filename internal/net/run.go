// Package net implements the commands that open a socket: curl, wget, ssh and
// scp. They live in their own package so that a build without networking can
// leave them out entirely (acmesh-mq1, and the nonet build tag).
package net

import (
	"fmt"
	"os"
)

// CmdFunc is the shape every command in this package has.
type CmdFunc func(args []string, stdin *os.File, stdout, stderr *os.File) int

// Run executes one of the network commands.
func Run(cmd string, args []string, stdin *os.File, stdout, stderr *os.File) int {
	fn, ok := handlers[cmd]
	if !ok {
		fmt.Fprintf(stderr, "%s: not available in this build\n", cmd)
		return 2
	}
	return fn(args, stdin, stdout, stderr)
}
