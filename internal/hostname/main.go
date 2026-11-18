// Package hostname implements hostname, printing or setting the name.

package hostname

import (
	"fmt"
	"os"
)

func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	if len(args) > 0 {
		fmt.Fprintln(stderr, "hostname: setting the name is not supported")
		return 2
	}
	host, err := os.Hostname()
	if err != nil {
		fmt.Fprintf(stderr, "hostname: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, host)
	return 0
}

// timeoutOptions is what GNU timeout accepts before the duration.
