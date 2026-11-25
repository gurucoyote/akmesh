// Command sh is the shell from internal/sh, reachable as its own
// command inside the akmesh binary.
package main

import (
	"os"

	"akmesh/internal/toolsh"

	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(toolsh.Run("sh"))
}
