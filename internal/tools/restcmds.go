package tools

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/clear"
	"github.com/gurucoyote/akmesh/internal/corecmd"
	"github.com/gurucoyote/akmesh/internal/env"
	"github.com/gurucoyote/akmesh/internal/file"
	"github.com/gurucoyote/akmesh/internal/iconv"
	"github.com/gurucoyote/akmesh/internal/policycmd"
)

// The last command implementations moved into their own packages in acmesh-mq1;
// these wrappers keep the command table intact, which is the single place that
// names every command.

func runClear(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return clear.Run(args, stdin, stdout, stderr)
}

func runEcho(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return corecmd.Run("echo", args, stdin, stdout, stderr)
}

func runPwd(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return corecmd.Run("pwd", args, stdin, stdout, stderr)
}

func runTrue(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return corecmd.Run("true", args, stdin, stdout, stderr)
}

func runFalse(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return corecmd.Run("false", args, stdin, stdout, stderr)
}

func runEnv(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return env.Run(args, stdin, stdout, stderr)
}

func runFile(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return file.Run(args, stdin, stdout, stderr)
}

func runIconv(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return iconv.Run(args, stdin, stdout, stderr)
}

func runPolicy(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return policycmd.Run("policy", args, stdin, stdout, stderr)
}
