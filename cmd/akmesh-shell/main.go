// Command akmesh-shell is the shell binary: sh, gosh and the apk commands that
// run package scripts through it.
package main

import (
	"os"

	"akmesh/internal/shellcmd"
)

func main() {
	os.Exit(shellcmd.Main())
}
