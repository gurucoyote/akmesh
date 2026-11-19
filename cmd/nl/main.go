// Command nl is the nl tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/nl and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/nl"

	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(nl.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
