// Command gosh is the gosh tool from internal/tools, reachable as its own
// command inside the akmesh binary.
package main

import (
	"os"

	"akmesh/internal/tools"

	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(tools.Run("gosh"))
}
