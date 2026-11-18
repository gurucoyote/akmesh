// Package corecmd implements echo, pwd, true and false: the four commands every
// script keeps using. They live together because each one is a few lines and
// they share the escape handling of echo.
package corecmd

import "os"

// Run executes one of the four commands.
func Run(cmd string, args []string, stdin *os.File, stdout, stderr *os.File) int {
	switch cmd {
	case "echo":
		return runEcho(args, stdin, stdout, stderr)
	case "pwd":
		return runPwd(args, stdin, stdout, stderr)
	case "true":
		return runTrue(args, stdin, stdout, stderr)
	case "false":
		return runFalse(args, stdin, stdout, stderr)
	}
	return 2
}
