package tools

import (
	"os"

	"akmesh/internal/sum"
)

// runSum keeps the four checksum commands in the command table. The
// implementation lives in internal/sum, so that a build carrying only one of
// them does not pull in the rest of this package (acmesh-mq1).
func runSum(kind string) Func {
	return func(args []string, stdin *os.File, stdout, stderr *os.File) int {
		return sum.Run(kind, args, stdin, stdout, stderr)
	}
}
