// Command gosh is the gosh tool from internal/tools, reachable as its own
// command inside the akmesh binary.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/toolsh"

	"github.com/gurucoyote/akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(toolsh.Run("gosh"))
}
