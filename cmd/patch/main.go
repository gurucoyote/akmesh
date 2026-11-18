// Command patch is the patch tool, reachable as its own command inside the acmesh
// binary.
//
// It imports only internal/diffpatch and not internal/tools (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/diffpatch"
	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(diffpatch.Run("patch", os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
