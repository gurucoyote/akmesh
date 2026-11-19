// Command httpd serves a directory over HTTP or HTTPS, reachable as its own
// name inside the akmesh binary.
package main

import (
	"os"

	"akmesh/internal/httpd"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(httpd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
