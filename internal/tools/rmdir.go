package tools

import (
	"os"

	"akmesh/internal/rmdir"
)

func runRmdir(args []string, stdin, stdout, stderr *os.File) int {
	return rmdir.Run("rmdir", args, stdout, stderr)
}
