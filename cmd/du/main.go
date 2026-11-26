// Command du is the du tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/disk and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/disk"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamed("du", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, disk.Run))
}
