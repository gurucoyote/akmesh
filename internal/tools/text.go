package tools

import (
	"os"

	"acmesh/internal/basename"
	"acmesh/internal/cut"
	"acmesh/internal/dirname"
	"acmesh/internal/nl"
	"acmesh/internal/tac"
)

// The small text commands live in their own packages since acmesh-mq1; these
// wrappers keep them in the command table, so the tests and the table stay one
// place while a build that carries only one of them carries only that package.

func runBasename(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return basename.Run(args, stdin, stdout, stderr)
}

func runDirname(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return dirname.Run(args, stdin, stdout, stderr)
}

func runNl(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return nl.Run(args, stdin, stdout, stderr)
}

func runTac(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return tac.Run(args, stdin, stdout, stderr)
}

func runCut(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return cut.Run(args, stdin, stdout, stderr)
}
