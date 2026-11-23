package tools

import (
	"os"
	"path/filepath"

	"akmesh/internal/corecmd"
	"akmesh/internal/diffpatch"
	"akmesh/internal/env"
	"akmesh/internal/file"
	"akmesh/internal/iconv"
	"akmesh/internal/jq"
	"akmesh/internal/policycmd"
	"akmesh/internal/sedawk"
	"akmesh/internal/sh"
)

// The last command implementations moved into their own packages in acmesh-mq1;
// these wrappers keep the command table intact, which is the single place that
// names every command.

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

func runJq(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return jq.Run(args, stdin, stdout, stderr)
}

func runSed(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return sedawk.Run("sed", args, stdin, stdout, stderr)
}

func runAwk(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return sedawk.Run("awk", args, stdin, stdout, stderr)
}

func runDiff(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return diffpatch.Run("diff", args, stdin, stdout, stderr)
}

func runPatch(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return diffpatch.Run("patch", args, stdin, stdout, stderr)
}

func runSh(args []string, stdin *os.File, stdout, stderr *os.File) int {
	// The shell answers $0 with the name it was called with, so the dispatcher's
	// binary path must not leak into the session (acmesh-51b). "gosh" stays
	// reachable under the name the old command list carried.
	name := "sh"
	if base := filepath.Base(os.Args[0]); base == "gosh" {
		name = "gosh"
	}
	return sh.RunAs(name, args, stdin, stdout, stderr)
}

func runPolicy(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return policycmd.Run("policy", args, stdin, stdout, stderr)
}
