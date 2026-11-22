// Command login starts a session as another user, reachable as its own command
// inside the akmesh binary.
package main

import (
	"os"

	"akmesh/internal/login"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(login.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
