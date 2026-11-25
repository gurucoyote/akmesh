package tools

import (
	"os"

	"akmesh/internal/diffpatch"
	"akmesh/internal/forked/tar"
	"akmesh/internal/sedawk"
)

// The wrappers these tests call moved to internal/toolsed and internal/tooldiff
// (acmesh-rnyu), so that the base binary no longer links Go.Sed, goawk or
// go-gitdiff. The compatibility tests stay here - they compare behaviour against
// GNU, not the wiring - and reach the implementations through these shims.
func runSed(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return sedawk.Run("sed", args, stdin, stdout, stderr)
}

func runDiff(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return diffpatch.Run("diff", args, stdin, stdout, stderr)
}

func runPatch(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return diffpatch.Run("patch", args, stdin, stdout, stderr)
}

func runTar(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return tar.Run(args, stdin, stdout, stderr)
}
