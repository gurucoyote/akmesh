// Command policy reports on the policy the process runs under, reachable as its
// own command inside the acmesh binary.
//
// It imports only internal/policycmd and not internal/tools (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/policy"
	"acmesh/internal/policycmd"
)

func main() {
	policy.Enter()
	os.Exit(policycmd.Run("policy", os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
