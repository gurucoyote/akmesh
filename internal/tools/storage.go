package tools

import (
	"os"

	"akmesh/internal/disk"
)

// free, df and du live in internal/disk since acmesh-mq1; these wrappers keep
// them in the command table.

func runFree(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return disk.Run("free", args, stdin, stdout, stderr)
}

func runDf(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return disk.Run("df", args, stdin, stdout, stderr)
}

func runDu(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return disk.Run("du", args, stdin, stdout, stderr)
}
