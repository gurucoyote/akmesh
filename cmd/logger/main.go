// Command logger sends one message to /dev/log; on Windows it appends to a local
// file instead (acmesh-g7n).
package main

import (
	"os"

	"akmesh/internal/logger"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(logger.Run("logger", os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
