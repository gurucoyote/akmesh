// Command delgroup is the delgroup tool, reachable as its own command inside the
// akmesh binary.
//
// It imports only internal/useradd and not internal/tools: the table there pulls
// every tool we own into the binary (acmesh-mq1).
package main

import (
	"io"
	"os"

	"github.com/gurucoyote/akmesh/internal/useradd"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamedIO("delgroup", os.Args[1:], os.Stdin, os.Stdout, os.Stderr,
		func(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
			return useradd.Run(cmd, args, stdin, stdout, stderr)
		}))
}
