// Command sha512sum is the checksum tool, reachable as its own command inside the
// acmesh binary.
//
// It imports only internal/sum and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/sum"

	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(sum.Run("sha512", os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
