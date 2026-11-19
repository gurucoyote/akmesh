// Command split is the split tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/split and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/split"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.Run("split", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, split.Run))
}
