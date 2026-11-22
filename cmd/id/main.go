// Command id is the id tool, reachable as its own command inside the akmesh
// binary. It reports the numeric ids even when /etc/passwd is missing, which is
// the normal case in a minimal image.
package main

import (
	"os"

	"akmesh/internal/forked/id"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunIO("id", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, id.Run))
}
