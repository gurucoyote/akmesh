// Package sedawk implements sed and awk. They share the word and expression
// parsing, so they live in one package (acmesh-mq1).
package sedawk

import "os"

// Run executes sed or awk.
func Run(cmd string, args []string, stdin *os.File, stdout, stderr *os.File) int {
	switch cmd {
	case "sed":
		return runSed(args, stdin, stdout, stderr)
	case "awk":
		return runAwk(args, stdin, stdout, stderr)
	}
	return 2
}
