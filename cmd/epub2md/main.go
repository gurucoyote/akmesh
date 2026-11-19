// Command epub2md reads an EPUB book into Markdown.
package main

import (
	"os"

	"acmesh/internal/epubmd"
	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(epubmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
