// Package tools carries the commands that u-root either does not have or does
// not build for Windows: text tools, diff/patch, sed, awk, checksums and a few
// system queries.
//
// The implementation is one package with one table, so that the same code is
// reached two ways: as a command of the akmesh binary (one tiny main package
// per name, which calls Run) and from tests.
//
// Size matters here. See docs/groesse.md for the budget and the rules that keep
// this package small: standard library first, a small library only where the
// alternative would be a rewrite (Go.Sed, goawk, go-gitdiff).
package tools

import (
	"os"

	"akmesh/internal/cmdrun"
	"akmesh/internal/policy"
	"akmesh/internal/shuf"
	"akmesh/internal/sort"
	"akmesh/internal/usage"
)

// Func is the signature every command implements. It receives the arguments
// after the command name plus the standard streams, and returns the exit code.
// It is an alias so that a command's Run can be handed to the shared help gate
// without a conversion.
type Func = usage.Func

// Commands maps a command name to its implementation.
var Commands = map[string]Func{
	"base64":    runBase64,
	"basename":  runBasename,
	"chmod":     runChmod,
	"chown":     runChown,
	"cp":        runCp,
	"comm":      runComm,
	"cut":       runCut,
	"dirname":   runDirname,
	"echo":      runEcho,
	"false":     runFalse,
	"env":       runEnv,
	"df":        runDf,
	"file":      runFile,
	"find":      runFind,
	"grep":      runGrep,
	"head":      runHead,
	"hostname":  runHostname,
	"du":        runDu,
	"free":      runFree,
	"hexdump":   runHexdump,
	"iconv":     runIconv,
	"ls":        runLs,
	"kill":      runKill,
	"killall":   runKillall,
	"pgrep":     runPgrep,
	"pkill":     runPkill,
	"md5sum":    runSum("md5"),
	"nl":        runNl,
	"od":        runOdCommand,
	"passwd":    runPasswd,
	"adduser":   runAddUser,
	"addgroup":  runAddGroup,
	"deluser":   runDelUser,
	"delgroup":  runDelGroup,
	"rmdir":     runRmdir,
	"policy":    runPolicy,
	"blkid":     runBlkid,
	"ps":        runPS,
	"pwd":       runPwd,
	"sha1sum":   runSum("sha1"),
	"sha256sum": runSum("sha256"),
	"sha512sum": runSum("sha512"),
	"shuf":      shufcmd.Run,
	"sort":      sortcmd.Run,
	"split":     runSplit,
	"setsid":    runSetsid,
	"stat":      runStat,
	"su":        runSu,
	"sync":      runSync,
	"sum":       runBsdSum,
	"tac":       runTac,
	"timeout":   runTimeout,
	"truncate":  runTruncate,
	"true":      runTrue,
	"touch":     runTouch,
	"tr":        runTr,
	"uniq":      runUniq,
	"seq":       runSeq,
	"uname":     runUname,
	"id":        runID,
	"mkpasswd":  runMkpasswd,
	"unzip":     runUnzip,
	"wc":        runWc,
	"xargs":     runXargs,
}

// policyOnce makes sure the policy is applied exactly once per process, before
// any command runs. A broken policy stops the process: running unconfined after a
// typo would be the worst failure mode this binary could have.
// ensurePolicy keeps the table's own entry point behind the same rule as the
// per-command mains, which call policy.Enter in their main function.
func ensurePolicy() { policy.Enter() }

// Run executes the named command with the arguments the program was started
// with, minus argv[0]. It is the entry point for the per-command main packages.
func Run(name string) int {
	ensurePolicy()
	return cmdrun.Run(Commands, name)
}

// RunArgs executes one command the way the dispatcher does; see cmdrun.RunArgs.
func RunArgs(name string, args []string, stdin, stdout, stderr *os.File) int {
	return cmdrun.RunArgs(Commands, name, args, stdin, stdout, stderr)
}
