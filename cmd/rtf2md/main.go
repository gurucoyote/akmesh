// Command rtf2md is any2md with the format fixed to rtf. The reader lives in
// internal/anymd; this name stays so that a script that means "rtf" keeps
// saying so.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/anymd"
	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(anymd.RunAs("rtf2md", anymd.RTF, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
