package toolpkg

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/apkreport"
)

func runApkreport(args []string, stdin, stdout, stderr *os.File) int {
	return apkreport.Run(args, stdout, stderr)
}
