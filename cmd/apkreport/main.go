// Command apkreport writes one JSON line per installed package: what this host
// has, in which channel, and what the last update and health check said
// (acmesh-slyv). The file is the base for the rollback watcher.
package main

import (
	"io"
	"os"

	"akmesh/internal/apkreport"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamedIO("apkreport", os.Args[1:], os.Stdin, os.Stdout, os.Stderr,
		func(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
			return apkreport.Run(args, stdout, stderr)
		}))
}
