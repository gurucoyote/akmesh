// Command html2docx writes a Word document from HTML.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/docxgen"
	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(docxgen.RunHTML(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
