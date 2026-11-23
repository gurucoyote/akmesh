// Command logread prints the log file, optionally following it (acmesh-g7n).
package main

import (
	"os"

	"akmesh/internal/logread"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(logread.Run("logread", os.Args[1:], os.Stdout, os.Stderr))
}
