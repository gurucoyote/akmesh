// Command certinfo prints what a certificate says, from a file or from a host.
package main

import (
	"os"

	"akmesh/internal/certinfo"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.Run("certinfo", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, certinfo.Run))
}
