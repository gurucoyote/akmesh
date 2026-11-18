// Command df is the df tool, reachable as its own command inside the acmesh
// binary.
//
// It imports only internal/disk and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/disk"
)

func main() {
	os.Exit(disk.Run("df", os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
