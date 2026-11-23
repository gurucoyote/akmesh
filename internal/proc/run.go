// Package proc implements ps, kill, killall, pgrep and pkill: what an agent needs to look at
// processes and to stop them. The five share one process model, so they live
// in one package and pick the command by name (acmesh-mq1).
package proc

import "os"

// Run executes one of the three commands.
func Run(cmd string, args []string, stdin *os.File, stdout, stderr *os.File) int {
	switch cmd {
	case "ps":
		return runPS(args, stdin, stdout, stderr)
	case "kill":
		return runKill(args, stdin, stdout, stderr)
	case "killall":
		return runKillall(args, stdin, stdout, stderr)
	case "pgrep":
		return runPgrep(args, stdin, stdout, stderr)
	case "pkill":
		return runPkill(args, stdin, stdout, stderr)
	}
	return 2
}
