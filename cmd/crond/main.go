// Command crond runs time jobs: crontab lines, output on stdout, SIGHUP reloads
// (acmesh-qgf). It uses the same code on every target, so a crontab written on
// Linux runs on Windows.
package main

import (
	"os"

	"akmesh/internal/crond"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(crond.Run("crond", os.Args[1:], os.Stdout, os.Stderr))
}
