// Package diffpatch implements diff and patch. They share the diff engine, so a
// change to the hunk format touches both in one place (acmesh-mq1).
package diffpatch

import "os"

// Run executes diff or patch.
func Run(cmd string, args []string, stdin *os.File, stdout, stderr *os.File) int {
	switch cmd {
	case "diff":
		return runDiff(args, stdin, stdout, stderr)
	case "patch":
		return runPatch(args, stdin, stdout, stderr)
	}
	return 2
}
