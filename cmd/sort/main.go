// Command sort is the sort tool from internal/sort, reachable as its own
// command inside the akmesh binary.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/tools"

	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(tools.Run("sort"))
}
