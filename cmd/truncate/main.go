// Command truncate is the truncate tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/truncate and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/truncate"

	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(truncate.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
