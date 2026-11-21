// This file is copied into the generated multicall source by
// scripts/build-bb.sh. It is not a command: it is the one function the patched
// dispatcher calls when a name was not found, so that the list it prints is ours
// (internal/cmdlist) and not a foreign template.
//
// Why a copy and not an import: the generated module vendors a fixed set of
// packages, and a new import from akmesh/internal would not be in it. The file
// carries no akmesh import for that reason; the table itself is generated from
// internal/cmdlist/cmdlist.go with its package clause rewritten.
package main

import (
	"os"

	"bb.u-root.com/bb/pkg/bbmain"
)

// printSupported writes the command list on standard error, grouped.
func printSupported() {
	if err := Print(os.Stderr, bbmain.ListCmds()); err != nil {
		// The list is a courtesy after a failed dispatch; if even that cannot be
		// written, the exit code still says the call failed.
		return
	}
}
