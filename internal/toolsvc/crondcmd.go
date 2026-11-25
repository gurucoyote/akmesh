package toolsvc

import (
	"os"

	"akmesh/internal/crond"
)

// runCrond hands the dispatcher's call to the crond package.
func runCrond(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return crond.Run("crond", args, stdout, stderr)
}
