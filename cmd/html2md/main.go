// Command html2md turns an HTML fragment into Markdown, reachable as its own
// name inside the acmesh binary.
package main

import (
	"os"

	"acmesh/internal/htmlmd"
	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(htmlmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
