// Command killall is the killall tool, reachable as its own command inside the acmesh
// binary.
//
// It imports only internal/proc and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary (acmesh-mq1).
package main

import (
	"os"

	"acmesh/internal/proc"
)

func main() {
	os.Exit(proc.Run("killall", os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
