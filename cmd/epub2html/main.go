// Command epub2html reads an EPUB book into HTML.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/epubmd"
	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(epubmd.RunHTML(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
