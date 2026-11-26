// Command md2docx renders Markdown as a .docx.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/docxgen"
	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(docxgen.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
