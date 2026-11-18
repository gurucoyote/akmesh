// Command pdf2md prints the text of a PDF, reachable as its own name inside the
// acmesh binary.
package main

import (
	"os"

	"acmesh/internal/pdfmd"
	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(pdfmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
