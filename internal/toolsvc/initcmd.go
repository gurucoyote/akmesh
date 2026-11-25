package toolsvc

import (
	"os"

	"akmesh/internal/akinit"
)

// runInit hands the dispatcher's call to the init package: same binary, same
// policy, one entry point for "akmesh init".
func runInit(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return akinit.Run("init", args, stdout, stderr)
}
