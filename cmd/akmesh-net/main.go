// Command akmesh-net is the network binary: curl, wget, ssh, scp, httpd and the
// certificate commands.
package main

import (
	"os"

	"github.com/gurucoyote/akmesh/internal/netcmd"
)

func main() {
	os.Exit(netcmd.Main())
}
