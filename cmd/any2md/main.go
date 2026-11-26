// Command any2md reads a document and prints it as Markdown, whatever it is:
// PDF, docx, EPUB, RTF or HTML. The format comes from the bytes, so the caller
// does not have to know it; the five *2md commands are the same reader with the
// format pinned.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/anymd"
	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(anymd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
