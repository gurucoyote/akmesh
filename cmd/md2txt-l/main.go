// Command md2txt-l prints Markdown as plain text, reachable as its own name inside
// the akmesh binary.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/md2txt"
	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(md2txt.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
