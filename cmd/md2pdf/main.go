// Command md2pdf renders Markdown as a PDF.
package main

import (
	"os"

	"acmesh/internal/pdfgen"
	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(pdfgen.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
