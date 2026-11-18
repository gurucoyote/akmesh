// Command md2txt prints Markdown as plain text, reachable as its own name inside
// the acmesh binary.
package main

import (
	"os"

	"acmesh/internal/md2txt"
	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(md2txt.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
