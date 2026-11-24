// Command seq is the seq tool, reachable as its own command inside the
// akmesh binary.
//
// It imports only akmesh/internal/seq: the table in internal/tools
// pulls every tool we own into the binary (see acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/seq"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunIO("seq", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, seq.Run))
}
