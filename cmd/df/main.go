// Command df is the df tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/disk and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/disk"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamed("df", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, disk.Run))
}
