package toolsvc

import (
	"os"

	"akmesh/internal/login"
)

// runLogin is the glue between the command table and internal/login.
func runLogin(args []string, stdin *os.File, stdout, stderr *os.File) int {
	return login.Run(args, stdin, stdout, stderr)
}
