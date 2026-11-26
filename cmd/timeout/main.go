// Command timeout is the timeout tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/timeout and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/timeout"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.Run("timeout", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, timeout.Run))
}
