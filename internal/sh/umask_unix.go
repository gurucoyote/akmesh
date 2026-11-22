//go:build !windows

package sh

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"syscall"

	"mvdan.cc/sh/v3/interp"
)

// umaskCall answers the umask builtin. mvdan/sh has none, and the /etc/profile
// of a normal Linux calls it on every login: without this, every profile run
// printed "umask: unsupported builtin".
//
// It runs through CallHandler, which sees every simple command including
// builtins, and lets the shell carry on with a harmless "true". The mask is a
// process property, so it is set here and not in a child.
func umaskCall(out *switchWriter, stderr io.Writer) interp.CallHandlerFunc {
	return func(ctx context.Context, args []string) ([]string, error) {
		if len(args) == 0 || args[0] != "umask" {
			return args, nil
		}
		switch len(args) {
		case 1:
			// Reading the mask means setting it and putting it back.
			cur := setMask(0)
			setMask(cur)
			out.WriteString(fmt.Sprintf("%04o\n", cur))
			return []string{"true"}, nil
		case 2:
			if args[1] == "-S" || strings.ContainsAny(args[1], "rwxXstugo=") {
				// Symbolic modes are ash-specific and not implemented here.
				// Saying so is better than silently ignoring the mask.
				fmt.Fprintln(stderr, "umask: symbolic modes are not supported, use an octal mask")
				return []string{"false"}, nil
			}
			if args[1] == "-p" {
				out.WriteString(fmt.Sprintf("umask %04o\n", currentMask()))
				return []string{"true"}, nil
			}
			v, err := strconv.ParseUint(args[1], 8, 32)
			if err != nil {
				fmt.Fprintf(stderr, "umask: invalid mask %q\n", args[1])
				return []string{"false"}, nil
			}
			setMask(int(v))
			return []string{"true"}, nil
		default:
			fmt.Fprintln(stderr, "usage: umask [MASK]")
			return []string{"false"}, nil
		}
	}
}

// setMask sets the umask and returns the previous one.
func setMask(v int) int {
	return syscall.Umask(v)
}

// currentMask reads the mask without leaving a zero mask behind.
func currentMask() int {
	cur := setMask(0)
	setMask(cur)
	return cur
}
