//go:build windows

// Package blkid has no Windows implementation: block devices and superblocks are
// a Linux idea, and a guess would be worse than the honest answer.
package blkid

import (
	"fmt"
	"io"
)

// Run says so and fails, instead of printing nothing that looks like success.
func Run(name string, args []string, stdout, stderr io.Writer) int {
	fmt.Fprintf(stderr, "%s: not supported on Windows\n", name)
	return 1
}
