// Command md2txt-l prints Markdown as plain text, reachable as its own name inside
// the akmesh binary.
package main

import (
	"os"

	"akmesh/internal/md2txt"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(md2txt.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
