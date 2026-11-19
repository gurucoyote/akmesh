// Command shuf is the shuf tool from internal/shuf, reachable as its own
// command inside the akmesh binary.
package main

import (
	"os"

	"akmesh/internal/tools"

	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(tools.Run("shuf"))
}
