// Command mkcert issues test certificates and, on request, a small CA of its own.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/mkcert"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.Run("mkcert", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, mkcert.Run))
}
