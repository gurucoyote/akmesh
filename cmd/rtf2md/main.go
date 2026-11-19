// Command rtf2md turns an RTF document into Markdown, reachable as its own name
// inside the akmesh binary.
package main

import (
	"os"

	"akmesh/internal/policy"
	"akmesh/internal/rtfmd"
)

func main() {
	policy.Enter()
	os.Exit(rtfmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
