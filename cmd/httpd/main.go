// Command httpd serves a directory over HTTP or HTTPS, reachable as its own
// name inside the acmesh binary.
package main

import (
	"os"

	"acmesh/internal/httpd"
	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(httpd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
