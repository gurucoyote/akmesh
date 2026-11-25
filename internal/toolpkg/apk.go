package toolpkg

import (
	"os"

	"akmesh/internal/apkcmd"
)

func runApk(args []string, stdin, stdout, stderr *os.File) int {
	return apkcmd.Run("apk", args, stdin, stdout, stderr)
}
