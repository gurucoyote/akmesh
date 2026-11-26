// Command apkbuild builds apk packages and signed repository indexes in Go, the
// counterpart to apk: abuild is a shell script with fakeroot, this is not
// (acmesh-oolz).
package main

import (
	"io"
	"os"

	"github.com/gurucoyote/akmesh/internal/apkbuild"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamedIO("apkbuild", os.Args[1:], os.Stdin, os.Stdout, os.Stderr,
		func(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
			return apkbuild.Run(args, stdout, stderr)
		}))
}
