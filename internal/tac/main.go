// Package tac implements tac, concatenating files in reverse line order.

package tac

import (
	"akmesh/internal/shared"
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

func tacWithSeparator(files []string, sep string, useRegex bool, stdin *os.File, stdout, stderr *os.File) int {
	readers, code := shared.OpenInputs(files, stdin, stderr, "tac")
	if code != 0 {
		return code
	}
	defer shared.CloseInputs(readers)
	if sep == "" {
		sep = "\n"
	}
	var re *regexp.Regexp
	if useRegex {
		var err error
		re, err = regexp.Compile(sep)
		if err != nil {
			fmt.Fprintf(stderr, "tac: %v\n", err)
			return 2
		}
	}
	w := bufio.NewWriter(stdout)
	defer w.Flush()
	for _, r := range readers {
		data, err := io.ReadAll(r)
		if err != nil {
			fmt.Fprintf(stderr, "tac: %v\n", err)
			return 1
		}
		var records []string
		if re != nil {
			last := 0
			for _, m := range re.FindAllIndex(data, -1) {
				records = append(records, string(data[last:m[1]]))
				last = m[1]
			}
			if last < len(data) {
				records = append(records, string(data[last:]))
			}
		} else {
			rest := string(data)
			for {
				i := strings.Index(rest, sep)
				if i < 0 {
					break
				}
				records = append(records, rest[:i+len(sep)])
				rest = rest[i+len(sep):]
			}
			if rest != "" {
				records = append(records, rest)
			}
		}
		for i := len(records) - 1; i >= 0; i-- {
			fmt.Fprint(w, records[i])
		}
	}
	return 0
}

func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	// -s changes the record separator, -r makes it a regular expression. The
	// separator stays attached to its record, as in GNU tac.
	sep := ""
	useRegex := false
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-s" || a == "--separator":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "tac: option -s needs a value")
				return 2
			}
			i++
			sep = args[i]
		case strings.HasPrefix(a, "--separator="):
			sep = strings.TrimPrefix(a, "--separator=")
		case strings.HasPrefix(a, "-s") && len(a) > 2:
			sep = a[2:]
		case a == "-r" || a == "--regex":
			useRegex = true
		case strings.HasPrefix(a, "-") && a != "-" && a != "--":
			fmt.Fprintf(stderr, "tac: unknown option %q\n", a)
			return 2
		default:
			files = append(files, a)
		}
	}
	args = files
	if sep != "" || useRegex {
		return tacWithSeparator(args, sep, useRegex, stdin, stdout, stderr)
	}
	readers, code := shared.OpenInputs(args, stdin, stderr, "tac")
	if code != 0 {
		return code
	}
	defer shared.CloseInputs(readers)
	w := bufio.NewWriter(stdout)
	defer w.Flush()
	for _, r := range readers {
		lines, err := shared.ReadAllLines(r)
		if err != nil {
			fmt.Fprintf(stderr, "tac: %v\n", err)
			return 1
		}
		for i := len(lines) - 1; i >= 0; i-- {
			fmt.Fprintln(w, lines[i])
		}
	}
	return 0
}

// runCut implements cut -d DELIM -f LIST | cut -c LIST [FILE...]
// inSpans reports whether n falls into one of the cut ranges.
