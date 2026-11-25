// Command akmesh-net is the network binary: curl, wget, ssh, scp, httpd and the
// certificate commands.
package main

import (
	"os"

	"akmesh/internal/netcmd"
)

func main() {
	os.Exit(netcmd.Main())
}
