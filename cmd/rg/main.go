// Command rg searches files the way ripgrep does, recursive and
// gitignore-aware, with the flags an agent already knows.
package main

import (
	"os"

	"akmesh/internal/policy"
	"akmesh/internal/rgcmd"
	"akmesh/internal/usage"
)

func main() {
	policy.Enter()
	os.Exit(usage.RunIO("rg", os.Args[1:], os.Stdin, os.Stdout, os.Stderr, rgcmd.Run))
}
