package tools

// The commands whose implementation lives in a package of its own. The table
// here is what the multicall binary dispatches on; each of them also has a
// cmd/<name> main that imports only its own package (acmesh-mq1).

import (
	"os"

	"akmesh/internal/bsdsum"
	"akmesh/internal/chown"
	"akmesh/internal/login"
	"akmesh/internal/odump"
	"akmesh/internal/setsid"
	"akmesh/internal/unzip"
)

func runChown(args []string, stdin, stdout, stderr *os.File) int {
	return chown.Run("chown", args, stdin, stdout, stderr)
}

func runUnzip(args []string, stdin, stdout, stderr *os.File) int {
	return unzip.Run("unzip", args, stdin, stdout, stderr)
}

func runOdCommand(args []string, stdin, stdout, stderr *os.File) int {
	return odump.Run("od", args, stdin, stdout, stderr)
}

func runBsdSum(args []string, stdin, stdout, stderr *os.File) int {
	return bsdsum.Run("sum", args, stdin, stdout, stderr)
}

func runSetsid(args []string, stdin, stdout, stderr *os.File) int {
	return setsid.Run("setsid", args, stdin, stdout, stderr)
}

func runSu(args []string, stdin, stdout, stderr *os.File) int {
	return login.RunSu("su", args, stdin, stdout, stderr)
}

func runPasswd(args []string, stdin, stdout, stderr *os.File) int {
	return login.RunPasswd("passwd", args, stdin, stdout, stderr)
}

func runMkpasswd(args []string, stdin, stdout, stderr *os.File) int {
	return login.RunMkpasswd("mkpasswd", args, stdin, stdout, stderr)
}
