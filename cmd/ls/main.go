// Command ls lists directory contents the way GNU ls does.
package main

import (
	"os"

	"akmesh/internal/ls"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.Run("ls", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, ls.Run))
}
