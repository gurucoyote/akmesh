// Command apkguard upgrades, runs the health checks and rolls back once if they
// fail; after that it reports and stands still (acmesh-2p44).
package main

import (
	"io"
	"os"

	"akmesh/internal/apkguard"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamedIO("apkguard", os.Args[1:], os.Stdin, os.Stdout, os.Stderr,
		func(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
			return apkguard.Run(args, stdout, stderr)
		}))
}
