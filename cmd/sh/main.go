// Command sh is the shell from internal/sh, reachable as its own
// command inside the akmesh binary.
package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gurucoyote/akmesh/internal/policy"
	"github.com/gurucoyote/akmesh/internal/toolsh"
)

func main() {
	policy.Enter()
	// WSL starts the shell as a login shell, with argv[0] "-sh". The dispatcher
	// strips the dash to find the command, so the shell never saw it and read no
	// profile: no home directory, no PS1 (measured 25.11.2026). The dash is
	// passed on through the environment instead.
	if argv0 := filepath.Base(os.Args[0]); strings.HasPrefix(argv0, "-") {
		os.Setenv("AKMESH_SH_LOGIN", "1")
	}
	os.Exit(toolsh.Run("sh"))
}
