// Command dirname is the dirname tool, reachable as its own command inside the acmesh
// binary.
//
// It imports only internal/dirname and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/dirname"
)

func main() {
	os.Exit(dirname.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
