// Command xargs is the xargs tool, reachable as its own command inside the
// akmesh binary.
//
// It imports only akmesh/internal/forked/xargs and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary, and a build that
// carries one command should carry one command (see acmesh-mq1).
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/forked/xargs"

	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(xargs.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
