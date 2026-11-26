// Command uname is the uname tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/uname and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/uname"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.Run("uname", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, uname.Run))
}
