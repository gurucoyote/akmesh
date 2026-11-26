// Command nl is the nl tool, reachable as its own command inside the
// akmesh binary.
//
// It imports only akmesh/internal/nl: the table in internal/tools
// pulls every tool we own into the binary (see acmesh-mq1).
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/nl"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunIO("nl", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, nl.Run))
}
