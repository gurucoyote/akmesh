package tools

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/basename"
	"github.com/gurucoyote/akmesh/internal/cut"
	"github.com/gurucoyote/akmesh/internal/dirname"
	"github.com/gurucoyote/akmesh/internal/nl"
	"github.com/gurucoyote/akmesh/internal/seq"
	"github.com/gurucoyote/akmesh/internal/tac"
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

func runSeq(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return seq.Run(args, stdin, stdout, stderr)
}

func runCut(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return cut.Run(args, stdin, stdout, stderr)
}
