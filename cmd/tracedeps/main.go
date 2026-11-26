// Command tracedeps lists the files a program needs, reachable as its own name
// inside the akmesh binary.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/tracedeps"
)

func main() {
	policy.Enter()
	os.Exit(tracedeps.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
