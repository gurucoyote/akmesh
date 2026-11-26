// Command certinfo prints what a certificate says, from a file or from a host.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/certinfo"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.Run("certinfo", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, certinfo.Run))
}
