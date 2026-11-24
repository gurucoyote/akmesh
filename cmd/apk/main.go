// Command apk installs packages from signed repositories into a tree. It is
// our own implementation; apk-tools and the apko library are not linked in
// (measured, apko costs round 18.4 MB; acmesh-xwcm).
package main

import (
	"io"
	"os"

	"akmesh/internal/apkcmd"

	"akmesh/internal/policy"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunNamedIO("apk", os.Args[1:], os.Stdin, os.Stdout, os.Stderr,
		func(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
			return apkcmd.Run(cmd, args, stdin, stdout, stderr)
		}))
}
