// Package rmdir removes empty directories. The reference is GNU coreutils 9.7
// from the pinned image, not busybox: --ignore-fail-on-non-empty and the
// "failed to remove" wording are what an agent's scripts expect to read.
package rmdir

import (
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
)

type options struct {
	parents  bool
	verbose  bool
	ignoreNE bool // --ignore-fail-on-non-empty
}

// Run is the command entry point.
func Run(cmd string, args []string, stdout, stderr io.Writer) int {
	var o options
	var paths []string
	for _, a := range args {
		switch {
		case a == "--":
			continue
		case a == "-p" || a == "--parents":
			o.parents = true
		case a == "-v" || a == "--verbose":
			o.verbose = true
		case a == "--ignore-fail-on-non-empty":
			o.ignoreNE = true
		case len(a) > 1 && a[0] == '-' && a != "-":
			for _, c := range a[1:] {
				switch c {
				case 'p':
					o.parents = true
				case 'v':
					o.verbose = true
				default:
					if a[1] == '-' {
						fmt.Fprintf(stderr, "%s: unrecognized option '%s'\n", cmd, a)
					} else {
						fmt.Fprintf(stderr, "%s: invalid option -- '%c'\n", cmd, c)
					}
					fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", cmd)
					return 1
				}
			}
		default:
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		fmt.Fprintf(stderr, "%s: missing operand\n", cmd)
		fmt.Fprintf(stderr, "Try '%s --help' for more information.\n", cmd)
		return 1
	}
	status := 0
	for _, p := range paths {
		if !removeOne(cmd, p, o, stdout, stderr) {
			status = 1
		}
	}
	return status
}

// removeOne removes one operand, and with -p every parent that becomes empty.
// The two message forms are GNU's: the operand is quoted and bare, a parent in
// the walk is named after the word "directory".
func removeOne(cmd, path string, o options, stdout, stderr io.Writer) bool {
	ok := true
	if o.verbose {
		fmt.Fprintf(stdout, "%s: removing directory, '%s'\n", cmd, path)
	}
	if err := os.Remove(path); err != nil {
		if !(o.ignoreNE && isNotEmpty(err)) {
			fmt.Fprintf(stderr, "%s: failed to remove '%s': %s\n", cmd, path, reason(err))
			ok = false
		}
	}
	if !o.parents {
		return ok
	}
	for dir := parent(path); dir != "" && dir != "." && dir != "/"; dir = parent(dir) {
		if o.verbose {
			fmt.Fprintf(stdout, "%s: removing directory, '%s'\n", cmd, dir)
		}
		if err := os.Remove(dir); err != nil {
			if o.ignoreNE && isNotEmpty(err) {
				continue
			}
			fmt.Fprintf(stderr, "%s: failed to remove directory '%s': %s\n", cmd, dir, reason(err))
			return false
		}
	}
	return ok
}

func parent(path string) string {
	for i := len(path) - 1; i > 0; i-- {
		if path[i] == '/' {
			if i == 0 {
				return "/"
			}
			return path[:i]
		}
	}
	return "."
}

func isNotEmpty(err error) bool {
	return errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST)
}

// reason is the strerror text GNU prints after the colon.
func reason(err error) string {
	switch {
	case errors.Is(err, syscall.ENOENT):
		return "No such file or directory"
	case errors.Is(err, syscall.ENOTEMPTY):
		return "Directory not empty"
	case errors.Is(err, syscall.EEXIST):
		return "File exists"
	case errors.Is(err, syscall.ENOTDIR):
		return "Not a directory"
	case errors.Is(err, syscall.EACCES):
		return "Permission denied"
	case errors.Is(err, syscall.EBUSY):
		return "Device or resource busy"
	case errors.Is(err, syscall.EPERM):
		return "Operation not permitted"
	default:
		return err.Error()
	}
}
