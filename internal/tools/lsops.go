package tools

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/ls"
)

// ls lives in internal/ls since acmesh-wx6: the u-root command was patched, then
// re-implemented, because it refused -t/-r/-1/-i/-n/-A/-u/-c and printed the
// wrong date for old files.
func runLs(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return ls.Run(args, stdin, stdout, stderr)
}
