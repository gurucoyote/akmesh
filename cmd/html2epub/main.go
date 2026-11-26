// Command html2epub writes an EPUB 3 book from HTML.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/epubgen"
	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(epubgen.RunHTML(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
