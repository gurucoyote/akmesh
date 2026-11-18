// Command rtf2md turns an RTF document into Markdown, reachable as its own name
// inside the acmesh binary.
package main

import (
	"os"

	"acmesh/internal/policy"
	"acmesh/internal/rtfmd"
)

func main() {
	policy.Enter()
	os.Exit(rtfmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
