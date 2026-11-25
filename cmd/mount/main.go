// Command mount is the mount tool. Linux only: the Windows build leaves the
// command out, as it did when u-root supplied it.
package main

import (
	"os"

	"akmesh/internal/mount"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamed("mount", os.Args[1:], os.Stdin, os.Stdout, os.Stderr,
		func(_ string, args []string, stdin, stdout, stderr *os.File) int {
			return mount.Run(args, stdin, stdout, stderr)
		}))
}
