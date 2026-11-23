// Command init is PID 1 in a container: services from /etc/inittab, reaped
// children, forwarded signals (acmesh-b97).
//
// It imports internal/akinit and not internal/tools: the table there would pull
// every tool we own into the binary.
package main

import (
	"os"

	"akmesh/internal/akinit"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(akinit.Run("init", os.Args[1:], os.Stdout, os.Stderr))
}
