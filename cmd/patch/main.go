// Command patch is the patch tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/diffpatch and not internal/tools (acmesh-mq1).
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/diffpatch"
	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamed("patch", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, diffpatch.Run))
}
