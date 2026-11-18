// Command sync is the sync tool, reachable as its own command inside the acmesh
// binary.
//
// It imports only internal/sync and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/sync"

	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(sync.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
