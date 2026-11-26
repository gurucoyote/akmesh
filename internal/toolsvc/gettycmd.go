package toolsvc

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/getty"
)

// runGetty hands the dispatcher's call to the getty package.
func runGetty(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return getty.Run(args, stdin, stdout, stderr)
}
