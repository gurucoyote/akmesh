// Command ext is the standalone multicall binary for the commands in
// internal/tools. The combined busybox reaches the same code through one small
// main package per command; this binary exists so the tools can be built and
// tested on their own.
package main

import (
	"os"

	"acmesh/internal/tools"
)

func main() {
	os.Exit(tools.Main(os.Args[1:]))
}
