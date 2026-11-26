// Command docx2md is any2md with the format fixed to docx. The reader lives in
// internal/anymd; this name stays so that a script that means "docx" keeps
// saying so.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/anymd"
	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(anymd.RunAs("docx2md", anymd.DOCX, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
