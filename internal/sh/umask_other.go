//go:build windows

package sh

import (
	"context"
	"fmt"
	"io"

	"mvdan.cc/sh/v3/interp"
)

// umaskCall keeps the builtin's name visible on Windows, where there is no
// umask: the command says so instead of failing as an unknown name.
func umaskCall(out *switchWriter, stderr io.Writer) interp.CallHandlerFunc {
	return func(ctx context.Context, args []string) ([]string, error) {
		if len(args) > 0 && args[0] == "umask" {
			fmt.Fprintln(stderr, "umask: not supported on Windows")
			return []string{"false"}, nil
		}
		return args, nil
	}
}
