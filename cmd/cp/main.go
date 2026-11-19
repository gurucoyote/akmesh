// Command cp is the cp tool, reachable as its own command inside the
// akmesh binary.
//
// It imports only akmesh/internal/forked/cp and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary, and a build that
// carries one command should carry one command (see acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/forked/cp"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunIO("cp", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, cp.Run))
}
