package toolpkg

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/apkbuild"
)

func runApkbuild(args []string, stdin, stdout, stderr *os.File) int {
	return apkbuild.Run(args, stdout, stderr)
}
