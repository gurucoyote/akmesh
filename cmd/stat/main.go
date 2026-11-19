// Command stat is the stat tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/stat and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/stat"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.Run("stat", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, stat.Run))
}
