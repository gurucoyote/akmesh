// Command md2epub renders Markdown as an EPUB 3 book.
package main

import (
	"os"

	"acmesh/internal/epubgen"
	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(epubgen.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
