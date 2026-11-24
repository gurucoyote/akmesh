// Command deluser is the deluser tool, reachable as its own command inside the
// akmesh binary.
//
// It imports only internal/useradd and not internal/tools: the table there pulls
// every tool we own into the binary (acmesh-mq1).
package main

import (
	"io"
	"os"

	"akmesh/internal/useradd"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamedIO("deluser", os.Args[1:], os.Stdin, os.Stdout, os.Stderr,
		func(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
			return useradd.Run(cmd, args, stdin, stdout, stderr)
		}))
}
