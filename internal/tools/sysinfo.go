package tools

import (
	"akmesh/internal/blkid"
	"os"

	"akmesh/internal/hostname"
	"akmesh/internal/proc"
	"akmesh/internal/timeout"
	"akmesh/internal/uname"
)

// uname, hostname, ps, kill, killall and timeout live in their own packages
// since acmesh-mq1; these wrappers keep them in the command table.

func runUname(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return uname.Run(args, stdin, stdout, stderr)
}

func runHostname(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return hostname.Run(args, stdin, stdout, stderr)
}

func runBlkid(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return blkid.Run("blkid", args, stdout, stderr)
}

func runPS(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return proc.Run("ps", args, stdin, stdout, stderr)
}

func runKill(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return proc.Run("kill", args, stdin, stdout, stderr)
}

func runKillall(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return proc.Run("killall", args, stdin, stdout, stderr)
}

func runPgrep(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return proc.Run("pgrep", args, stdin, stdout, stderr)
}

func runPkill(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return proc.Run("pkill", args, stdin, stdout, stderr)
}

func runTimeout(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return timeout.Run(args, stdin, stdout, stderr)
}
