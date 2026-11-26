// Command httpd serves a directory over HTTP or HTTPS, reachable as its own
// name inside the akmesh binary.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/httpd"
	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(httpd.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
