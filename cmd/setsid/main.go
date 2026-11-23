// Command setsid is the setsid tool, reachable as its own command inside the
// akmesh binary.
//
// It imports only internal/setsid and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/setsid"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamed("setsid", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, setsid.Run))
}
