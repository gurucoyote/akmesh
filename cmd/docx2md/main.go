// Command docx2md turns a .docx into Markdown, reachable as its own name inside
// the acmesh binary.
package main

import (
	"os"

	"acmesh/internal/docxmd"
	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(docxmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
