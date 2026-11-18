// Command true is the true tool, reachable as its own command inside the acmesh
// binary.
//
// It imports only internal/corecmd and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/corecmd"
	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(corecmd.Run("true", os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
