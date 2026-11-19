// Command docx2md turns a .docx into Markdown, reachable as its own name inside
// the akmesh binary.
package main

import (
	"os"

	"akmesh/internal/docxmd"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(docxmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
