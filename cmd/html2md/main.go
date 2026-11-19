// Command html2md turns an HTML fragment into Markdown, reachable as its own
// name inside the akmesh binary.
package main

import (
	"os"

	"akmesh/internal/htmlmd"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(htmlmd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
