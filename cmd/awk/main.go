// Command awk is the awk tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/sedawk and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/sedawk"
	"github.com/gurucoyote/akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamed("awk", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, sedawk.Run))
}
