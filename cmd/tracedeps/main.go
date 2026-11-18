// Command tracedeps lists the files a program needs, reachable as its own name
// inside the acmesh binary.
package main

import (
	"os"

	"acmesh/internal/policy"
	"acmesh/internal/tracedeps"
)

func main() {
	policy.Enter()
	os.Exit(tracedeps.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
