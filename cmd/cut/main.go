// Command cut is the cut tool, reachable as its own command inside the acmesh
// binary.
//
// It imports only internal/cut and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/cut"

	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(cut.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
