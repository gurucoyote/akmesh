package toolpkg

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/apkguard"
)

func runApkguard(args []string, stdin, stdout, stderr *os.File) int {
	return apkguard.Run(args, stdout, stderr)
}
