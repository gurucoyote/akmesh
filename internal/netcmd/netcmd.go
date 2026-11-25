// Package netcmd is the command table of akmesh-net: the commands that talk to
// a network, from curl to the certificate checks.
//
// Why its own binary: TLS (utls, x/crypto), brotli and the compression
// libraries are the most expensive block after the document writers, and a
// harness that only reads files should not pay for them (docs/design/split.md).
package netcmd

import (
	"os"

	"akmesh/internal/certinfo"
	"akmesh/internal/certverify"
	"akmesh/internal/cmdrun"
	"akmesh/internal/httpd"
	"akmesh/internal/mkcert"
	"akmesh/internal/multicall"
	"akmesh/internal/net"
	"akmesh/internal/usage"
)

// viaNet adapts the one Run that serves curl, wget, ssh and scp.
func viaNet(name string) cmdrun.Func {
	return func(args []string, stdin, stdout, stderr *os.File) int {
		return net.Run(name, args, stdin, stdout, stderr)
	}
}

var Prog = multicall.Prog{
	Name: "akmesh-net",
	Commands: map[string]cmdrun.Func{
		"curl": func(args []string, stdin, stdout, stderr *os.File) int {
			return usage.Run("curl", args, stdin, stdout, stderr, viaNet("curl"))
		},
		"wget": func(args []string, stdin, stdout, stderr *os.File) int {
			return usage.Run("wget", args, stdin, stdout, stderr, viaNet("wget"))
		},
		"ssh": func(args []string, stdin, stdout, stderr *os.File) int {
			return usage.Run("ssh", args, stdin, stdout, stderr, viaNet("ssh"))
		},
		"scp": func(args []string, stdin, stdout, stderr *os.File) int {
			return usage.Run("scp", args, stdin, stdout, stderr, viaNet("scp"))
		},
		"httpd": httpd.Run,
		"mkcert": func(args []string, stdin, stdout, stderr *os.File) int {
			return usage.Run("mkcert", args, stdin, stdout, stderr, mkcert.Run)
		},
		"verify": func(args []string, stdin, stdout, stderr *os.File) int {
			return usage.Run("verify", args, stdin, stdout, stderr, certverify.Run)
		},
		"certinfo": func(args []string, stdin, stdout, stderr *os.File) int {
			return usage.Run("certinfo", args, stdin, stdout, stderr, certinfo.Run)
		},
	},
	Others: []string{"akmesh", "akmesh-docs", "akmesh-write", "akmesh-shell"},
}

// Main is the entry point of the binary, called from cmd/akmesh-net.
func Main() int { return Prog.Main() }

// Run is the dispatch without the policy, so a test sees what a caller sees.
func Run(argv []string, stdin, stdout, stderr *os.File) int {
	return Prog.Run(argv, stdin, stdout, stderr)
}

// Names lists this binary's commands.
func Names() []string { return Prog.Names() }
