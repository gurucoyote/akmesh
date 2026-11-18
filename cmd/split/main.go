// Command split is the split tool, reachable as its own command inside the acmesh
// binary.
//
// It imports only internal/split and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/split"

	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(split.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
