// Package toolsvc carries the commands of a booted system: init, getty and
// login for the console, crond, syslogd with logger and logread.
//
// Why its own block: the daemons bring robfig/cron and the syslog loop with
// them, measured on 25.11.2026 as about 1.45 MiB for crond alone against a
// binary that only has cat. A harness that wants file and text tools should not
// pay for a cron daemon, and a build that drops the block drops the whole
// console and daemon set in one line (docs/design/split.md, acmesh-rk8i).
//
// The group matches the one the command list prints as "Services".
package toolsvc

import (
	"github.com/gurucoyote/akmesh/internal/cmdrun"
)

// Commands is this block's table.
var Commands = map[string]cmdrun.Func{
	"init":    runInit,
	"getty":   runGetty,
	"login":   runLogin,
	"crond":   runCrond,
	"syslogd": runSyslogd,
	"logger":  runLogger,
	"logread": runLogread,
}

// Run is the entry point for cmd/<name>/main.go.
func Run(name string) int { return cmdrun.Run(Commands, name) }
