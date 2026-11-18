// Command ps is the ps tool, reachable as its own command inside the acmesh
// binary.
//
// It imports only internal/proc and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/proc"

	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(proc.Run("ps", os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
