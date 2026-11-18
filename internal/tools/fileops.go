package tools

import (
	"os"

	"acmesh/internal/split"
	"acmesh/internal/stat"
	"acmesh/internal/sync"
	"acmesh/internal/truncate"
)

// stat, truncate, split and sync live in their own packages since acmesh-mq1;
// these wrappers keep them in the command table.

func runStat(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return stat.Run(args, stdin, stdout, stderr)
}

func runTruncate(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return truncate.Run(args, stdin, stdout, stderr)
}

func runSplit(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return split.Run(args, stdin, stdout, stderr)
}

func runSync(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return sync.Run(args, stdin, stdout, stderr)
}
