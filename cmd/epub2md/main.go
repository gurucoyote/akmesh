// Command epub2md is any2md with the format fixed to epub. The reader lives in
// internal/anymd; this name stays so that a script that means "epub" keeps
// saying so.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/anymd"
	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(anymd.RunAs("epub2md", anymd.EPUB, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
