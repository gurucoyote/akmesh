// Command policy is the policy tool from internal/tools, reachable as its own
// command inside the combined busybox.
package main

import (
	"os"

	"acmesh/internal/tools"
)

func main() {
	os.Exit(tools.Run("policy"))
}
