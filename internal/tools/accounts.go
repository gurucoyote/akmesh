package tools

// The account commands. They share one package because adduser and deluser
// work on the same files; the table still names each one, and each cmd/<name>
// main imports only internal/useradd.

import (
	"os"

	"akmesh/internal/useradd"
)

func runAddUser(args []string, stdin, stdout, stderr *os.File) int {
	return useradd.Run("adduser", args, stdin, stdout, stderr)
}

func runAddGroup(args []string, stdin, stdout, stderr *os.File) int {
	return useradd.Run("addgroup", args, stdin, stdout, stderr)
}

func runDelUser(args []string, stdin, stdout, stderr *os.File) int {
	return useradd.Run("deluser", args, stdin, stdout, stderr)
}

func runDelGroup(args []string, stdin, stdout, stderr *os.File) int {
	return useradd.Run("delgroup", args, stdin, stdout, stderr)
}
