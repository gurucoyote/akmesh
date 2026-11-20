// Command html2docx writes a Word document from HTML.
package main

import (
	"os"

	"akmesh/internal/docxgen"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(docxgen.RunHTML(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
