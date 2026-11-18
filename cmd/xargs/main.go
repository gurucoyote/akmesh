// Command xargs is the xargs tool, reachable as its own command inside the
// acmesh binary.
//
// It imports only acmesh/internal/forked/xargs and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary, and a build that
// carries one command should carry one command (see acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/forked/xargs"
)

func main() {
	os.Exit(xargs.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
