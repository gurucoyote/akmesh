// Command hostname is the hostname tool, reachable as its own command inside the acmesh
// binary.
//
// It imports only internal/hostname and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/hostname"
)

func main() {
	os.Exit(hostname.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
