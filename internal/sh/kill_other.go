//go:build windows

package sh

import (
	"context"
	"fmt"
	"io"

	"mvdan.cc/sh/v3/interp"
)

// killCall answers the kill builtin on Windows. mvdan/sh answers "unsupported
// builtin" for it, and there are no POSIX signals here: saying so is better than
// pretending to have killed something.
func killCall(out *switchWriter, stderr io.Writer) interp.CallHandlerFunc {
	return func(ctx context.Context, args []string) ([]string, error) {
		if len(args) == 0 || args[0] != "kill" {
			return args, nil
		}
		fmt.Fprintln(stderr, "kill: not supported on Windows (no POSIX signals)")
		return []string{"false"}, nil
	}
}
