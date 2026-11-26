//go:build !nonet

package toolnet

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/net"
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

// Commands is filled in init: the certificate commands live in table.go, and
// these four are absent from a nonet build entirely.
func init() {
	Commands["curl"] = runCurl
	Commands["scp"] = runScp
	Commands["ssh"] = runSSH
	Commands["wget"] = runWget
}
