// Command blkid names the filesystem on a block device, the way BusyBox does.
//
// It imports only internal/blkid and not internal/tools: the table in
// internal/tools pulls every tool we own into the binary.
package main

import (
	"os"

	"akmesh/internal/blkid"
	"akmesh/internal/policy"
)

func main() {
	policy.Enter()
	os.Exit(blkid.Run("blkid", os.Args[1:], os.Stdout, os.Stderr))
}
