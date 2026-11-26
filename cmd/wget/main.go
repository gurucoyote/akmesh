// Command wget is the wget tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/net and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/net"

	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(net.Run("wget", os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
