// Command crond is part of the services block; see internal/toolsvc for why the
// daemons and the console sit together.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/toolsvc"
)

func main() {
	os.Exit(toolsvc.Run("crond"))
}
