// Command ls lists directory contents the way GNU ls does.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/ls"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.Run("ls", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, ls.Run))
}
