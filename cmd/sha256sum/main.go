// Command sha256sum is the checksum tool, reachable as its own command inside the
// akmesh binary.
//
// It imports only internal/sum and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/sum"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamedIO("sha256sum", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, sum.RunCommand))
}
