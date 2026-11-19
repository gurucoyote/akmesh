// Command tac is the tac tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/tac and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/tac"

	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(tac.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
