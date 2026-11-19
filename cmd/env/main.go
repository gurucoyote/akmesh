// Command env is the env tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/env and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/env"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(env.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
