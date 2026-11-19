// Command hexdump is the hexdump tool, reachable as its own command inside the
// akmesh binary.
//
// It imports only akmesh/internal/forked/hexdump and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary, and a build that
// carries one command should carry one command (see acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/forked/hexdump"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunIO("hexdump", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, hexdump.Run))
}
