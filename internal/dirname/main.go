// Package dirname implements dirname, the directory part of a name.

package dirname

import (
	"fmt"
	"os"
	"strings"
)

// Run implements dirname NAME...
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	// -z/--zero separates the results with NUL, for "xargs -0" chains.
	sep := "\n"
	var kept []string
	for _, a := range args {
		switch a {
		case "-z", "--zero":
			sep = "\x00"
		default:
			if strings.HasPrefix(a, "-") && a != "-" && a != "--" {
				fmt.Fprintf(stderr, "dirname: unknown option %q\n", a)
				return 2
			}
			kept = append(kept, a)
		}
	}
	args = kept
	if len(args) == 0 {
		fmt.Fprintln(stderr, "dirname: missing operand")
		return 2
	}
	for _, name := range args {
		clean := strings.TrimRight(name, "/\\")
		if clean == "" {
			fmt.Fprintf(stdout, ".%s", sep)
			continue
		}
		i := strings.LastIndexAny(clean, "/\\")
		if i < 0 {
			fmt.Fprintf(stdout, ".%s", sep)
			continue
		}
		dir := clean[:i]
		if dir == "" {
			dir = "/"
		}
		fmt.Fprintf(stdout, "%s%s", dir, sep)
	}
	return 0
}
