// Command comm is the comm tool, reachable as its own command inside the
// akmesh binary.
//
// It imports only akmesh/internal/forked/comm and not internal/tools: the table
// in internal/tools pulls every tool we own into the binary, and a build that
// carries one command should carry one command (see acmesh-mq1).
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/forked/comm"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunIO("comm", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, comm.Run))
}
