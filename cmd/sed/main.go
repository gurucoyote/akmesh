// Command sed is the sed tool, reachable as its own command inside the acmesh
// binary.
//
// It imports only internal/sedawk and not internal/tools (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/policy"
	"acmesh/internal/sedawk"
)

func main() {
	policy.Enter()
	os.Exit(sedawk.Run("sed", os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
