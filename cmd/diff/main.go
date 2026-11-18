// Command diff is the diff tool from internal/tools, reachable as its own
// command inside the acmesh binary.
package main

import (
	"os"

	"acmesh/internal/tools"

	"acmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(tools.Run("diff"))
}
