// Command free is the free tool, reachable as its own command inside the acmesh
// binary.
//
// It imports only internal/disk and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/disk"

	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(disk.Run("free", os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
