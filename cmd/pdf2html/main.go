// Command pdf2html reads a PDF into HTML.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/pdfmd"
	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(pdfmd.RunHTML(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
