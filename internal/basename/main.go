// Package basename implements basename, the name without its directory.

package basename

import (
	"fmt"
	"os"
	"strings"
)

// Run implements basename [-a] [-s SUFFIX] NAME...
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var suffix string
	var names []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-a", "--multiple":
			// accepted, behaviour is the same
		case "-s", "--suffix":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "basename: option needs an argument")
				return 2
			}
			i++
			suffix = args[i]
		case "-z", "--zero":
			fmt.Fprintln(stderr, "basename: -z is not supported")
			return 2
		default:
			names = append(names, args[i])
		}
	}
	if len(names) == 0 {
		fmt.Fprintln(stderr, "basename: missing operand")
		return 2
	}
	// basename NAME SUFFIX: the second bare operand is a suffix, not a name.
	if len(names) == 2 && suffix == "" {
		suffix = names[1]
		names = names[:1]
	}
	for _, name := range names {
		base := name
		if i := strings.LastIndexAny(base, "/\\"); i >= 0 {
			base = base[i+1:]
		}
		if base == "" {
			base = string([]rune{'/'})
		}
		if suffix != "" && base != suffix && strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
		}
		fmt.Fprintln(stdout, base)
	}
	return 0
}
