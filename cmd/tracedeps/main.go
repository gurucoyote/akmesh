// Command tracedeps lists the files a program needs, reachable as its own name
// inside the akmesh binary.
package main

import (
	"os"

	"akmesh/internal/policy"
	"akmesh/internal/tracedeps"
)

func main() {
	policy.Enter()
	os.Exit(tracedeps.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
