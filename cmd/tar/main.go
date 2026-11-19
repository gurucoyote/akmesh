// Command tar is the tar tool, reachable as its own command inside the
// akmesh binary.
//
// It imports only akmesh/internal/forked/tar and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary, and a build that
// carries one command should carry one command (see acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/forked/tar"

	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(tar.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
