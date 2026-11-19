// Command sed is the sed tool, reachable as its own command inside the akmesh
// binary.
//
// It imports only internal/sedawk and not internal/tools (acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/policy"
	"akmesh/internal/sedawk"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamed("sed", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, sedawk.Run))
}
