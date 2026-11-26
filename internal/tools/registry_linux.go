//go:build linux

package tools

import "github.com/gurucoyote/akmesh/internal/mount"

// mount is ours and not u-root's (acmesh-2p6w): WSL starts its distribution
// with "/bin/mount --internal-only -t drvfs", and drvfs has to be handed to
// WSL's own /init. Registered here so the Windows binary keeps the command list
// it had - mount is a Linux-only command, as it was before.
func init() {
	Commands["mount"] = mount.Run
}
