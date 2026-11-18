// Command shuf is the shuf tool from internal/shuf, reachable as its own
// command inside the acmesh binary.
package main

import (
	"os"

	"acmesh/internal/tools"
)

func main() {
	os.Exit(tools.Run("shuf"))
}
