// Package disk implements free, df and du: the space numbers an agent reads
// before writing or copying. The three share the same platform files, so they
// live in one package and pick the command by name (acmesh-mq1).
package disk

import "os"

// Run executes one of the three commands.
func Run(cmd string, args []string, stdin *os.File, stdout, stderr *os.File) int {
	switch cmd {
	case "free":
		return runFree(args, stdin, stdout, stderr)
	case "df":
		return runDf(args, stdin, stdout, stderr)
	case "du":
		return runDu(args, stdin, stdout, stderr)
	}
	return 2
}
