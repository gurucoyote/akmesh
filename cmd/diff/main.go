// Command diff is the diff tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/diffpatch and not internal/tools (acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/diffpatch"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(diffpatch.Run("diff", os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
