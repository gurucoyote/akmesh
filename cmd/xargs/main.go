// Command xargs is the xargs tool from internal/tools, reachable as its own
// command inside the acmesh binary.
package main

import (
	"os"

	"acmesh/internal/tools"
)

func main() {
	os.Exit(tools.Run("xargs"))
}
