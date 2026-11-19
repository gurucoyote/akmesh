// Command mkcert issues test certificates and, on request, a small CA of its own.
package main

import (
	"os"

	"akmesh/internal/mkcert"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.Run("mkcert", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, mkcert.Run))
}
