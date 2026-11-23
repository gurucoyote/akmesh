// Command getty owns a terminal: it prints /etc/issue, asks for the user name and
// starts login. init respawns it after a logout (acmesh-bu1).
package main

import (
	"os"

	"akmesh/internal/getty"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(getty.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
