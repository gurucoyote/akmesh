//go:build !nonet

package tools

import (
	"os"

	"acmesh/internal/net"
)

// The network commands live in internal/net since acmesh-mq1. They are still
// registered in init and still behind the nonet tag: a build with the tag must
// not link net/http, TLS and the ssh library at all.
func runCurl(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return net.Run("curl", args, stdin, stdout, stderr)
}

func runScp(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return net.Run("scp", args, stdin, stdout, stderr)
}

func runSSH(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return net.Run("ssh", args, stdin, stdout, stderr)
}

func runWget(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return net.Run("wget", args, stdin, stdout, stderr)
}

// netCommands is the set the registry test checks against cmd/<name>.
var netCommands = map[string]Func{
	"curl": runCurl,
	"scp":  runScp,
	"ssh":  runSSH,
	"wget": runWget,
}

func init() {
	for name, fn := range netCommands {
		Commands[name] = fn
	}
}
