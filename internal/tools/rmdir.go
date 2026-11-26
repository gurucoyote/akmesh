package tools

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/rmdir"
)

func runRmdir(args []string, stdin, stdout, stderr *os.File) int {
	return rmdir.Run("rmdir", args, stdout, stderr)
}
