// Command tac is the tac tool, reachable as its own command inside the
// akmesh binary.
//
// It imports only akmesh/internal/tac: the table in internal/tools
// pulls every tool we own into the binary (see acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/tac"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunIO("tac", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, tac.Run))
}
