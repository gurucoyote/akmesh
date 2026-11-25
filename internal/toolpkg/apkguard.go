package toolpkg

import (
	"os"

	"akmesh/internal/apkguard"
)

func runApkguard(args []string, stdin, stdout, stderr *os.File) int {
	return apkguard.Run(args, stdout, stderr)
}
