//go:build !nonet

package net

// netCommands are the tools that open a socket. They are registered in init so
// that a build with the "nonet" tag does not link net/http, TLS and the ssh
// library at all: the imports disappear, and with them megabytes of code.
var handlers = map[string]CmdFunc{
	"curl": runCurl,
	"scp":  runScp,
	"ssh":  runSSH,
	"wget": runWget,
}
