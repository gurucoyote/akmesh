//go:build !nonet

package tools

// netCommands are the tools that open a socket. They are registered in init so
// that a build with the "nonet" tag does not link net/http, TLS and the ssh
// library at all: the imports disappear, and with them megabytes of code.
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
