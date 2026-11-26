// Command jq is the jq tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/jq and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/jq"
	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(jq.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
