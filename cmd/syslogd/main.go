// Command syslogd writes /dev/log lines into a log file and rotates it
// (acmesh-g7n). It imports internal/syslogd and not internal/tools: the table
// there pulls every tool we own into the binary.
package main

import (
	"os"

	"akmesh/internal/policy"
	"akmesh/internal/syslogd"
)

func main() {
	policy.Enter()
	os.Exit(syslogd.Run("syslogd", os.Args[1:], os.Stdout, os.Stderr))
}
