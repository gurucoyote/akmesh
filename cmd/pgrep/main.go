// Command pgrep is the pgrep tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/proc and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/proc"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamed("pgrep", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, proc.Run))
}
