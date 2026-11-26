// Command pdf2md is any2md with the format fixed to pdf. The reader lives in
// internal/anymd; this name stays so that a script that means "pdf" keeps
// saying so.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/anymd"
	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(anymd.RunAs("pdf2md", anymd.PDF, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
