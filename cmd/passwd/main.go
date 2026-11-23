// Command passwd is the passwd tool, reachable as its own command inside the
// akmesh binary.
//
// It imports only internal/login and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"akmesh/internal/login"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamedIO("passwd", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, login.RunPasswd))
}
