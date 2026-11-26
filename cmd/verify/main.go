// Command verify checks a certificate chain against a CA or the system roots.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/certverify"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.Run("verify", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, certverify.Run))
}
