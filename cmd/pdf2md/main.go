// Command pdf2md prints the text of a PDF, reachable as its own name inside the
// akmesh binary.
package main

import (
	"os"

	"akmesh/internal/pdfmd"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(pdfmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
