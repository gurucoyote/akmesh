package toolnet

import "akmesh/internal/cmdrun"

// Commands is the network block's table: certificates here, the socket commands
// from netcmds.go (and absent in a nonet build).
//
// Why its own package: internal/net brings utls, x/crypto, brotli and
// klauspost/compress, which a file-and-text binary must not carry. Measured in
// docs/design/split.md; breaking the block out is acmesh-58me.
var Commands = map[string]cmdrun.Func{
	"certinfo": runCertinfo,
	"mkcert":   runMkcert,
	"verify":   runVerify,
}

// Run is the entry point for cmd/<name>/main.go.
func Run(name string) int { return cmdrun.Run(Commands, name) }
