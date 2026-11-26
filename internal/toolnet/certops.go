package toolnet

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/certinfo"
	"github.com/gurucoyote/akmesh/internal/certverify"
	"github.com/gurucoyote/akmesh/internal/mkcert"
)

// The certificate commands live in their own packages (acmesh-vcl, acmesh-pav,
// acmesh-0kh); these wrappers keep them in the command table.
func runCertinfo(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return certinfo.Run(args, stdin, stdout, stderr)
}

func runMkcert(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return mkcert.Run(args, stdin, stdout, stderr)
}

func runVerify(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return certverify.Run(args, stdin, stdout, stderr)
}
