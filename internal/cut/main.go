// Package cut implements cut, selecting fields, bytes and characters.

package cut

import (
	"akmesh/internal/shared"
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func inSpans(ranges []span, n int) bool {
	for _, sp := range ranges {
		if n >= sp.from && n <= sp.to {
			return true
		}
	}
	return false
}

func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var delim string
	var fields, chars string
	onlyDelimited := false
	bytesMode := false
	complement := false
	outDelim := ""
	var files []string
	getValue := func(i *int, attached string) (string, bool) {
		if attached != "" {
			return attached, true
		}
		if *i+1 >= len(args) {
			return "", false
		}
		*i++
		return args[*i], true
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		attached := ""
		switch {
		case strings.HasPrefix(a, "--delimiter="):
			a, attached = "-d", strings.TrimPrefix(a, "--delimiter=")
		case strings.HasPrefix(a, "--fields="):
			a, attached = "-f", strings.TrimPrefix(a, "--fields=")
		case strings.HasPrefix(a, "--characters="):
			a, attached = "-c", strings.TrimPrefix(a, "--characters=")
		case strings.HasPrefix(a, "--bytes="):
			a, attached = "-b", strings.TrimPrefix(a, "--bytes=")
		case strings.HasPrefix(a, "--output-delimiter="):
			a, attached = "-o", strings.TrimPrefix(a, "--output-delimiter=")
		case strings.HasPrefix(a, "--delimiter"):
			a = "-d"
		case strings.HasPrefix(a, "--fields"):
			a = "-f"
		case strings.HasPrefix(a, "--characters"):
			a = "-c"
		case len(a) > 2 && (a[:2] == "-d" || a[:2] == "-f" || a[:2] == "-c" || a[:2] == "-b"):
			a, attached = a[:2], a[2:]
		}
		switch a {
		case "-d":
			v, ok := getValue(&i, attached)
			if !ok {
				return 2
			}
			delim = v
			if len([]rune(delim)) != 1 {
				fmt.Fprintln(stderr, "cut: delimiter must be a single character")
				return 2
			}
		case "-f":
			v, ok := getValue(&i, attached)
			if !ok {
				return 2
			}
			fields = v
		case "-c", "-b":
			v, ok := getValue(&i, attached)
			if !ok {
				return 2
			}
			chars = v
			bytesMode = args[i] == "-b" || a == "-b"
		case "-o":
			v, ok := getValue(&i, attached)
			if !ok {
				return 2
			}
			outDelim = v
		case "--complement":
			complement = true
		case "-s", "--only-delimited":
			onlyDelimited = true
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				fmt.Fprintf(stderr, "cut: unknown option %q\n", args[i])
				return 2
			}
			files = append(files, args[i])
		}
	}
	if (fields == "") == (chars == "") {
		fmt.Fprintln(stderr, "cut: give exactly one of -f or -c")
		return 2
	}
	var ranges []span
	var err error
	if fields != "" {
		ranges, err = parseList(fields)
	} else {
		ranges, err = parseList(chars)
	}
	if err != nil {
		fmt.Fprintf(stderr, "cut: %v\n", err)
		return 2
	}
	if fields != "" && delim == "" {
		delim = "\t"
	}

	readers, code := shared.OpenInputs(files, stdin, stderr, "cut")
	if code != 0 {
		return code
	}
	defer shared.CloseInputs(readers)
	w := bufio.NewWriter(stdout)
	defer w.Flush()
	for _, r := range readers {
		lines, err := shared.ReadAllLines(r)
		if err != nil {
			fmt.Fprintf(stderr, "cut: %v\n", err)
			return 1
		}
		join := delim
		if outDelim != "" {
			join = outDelim
		}
		for _, line := range lines {
			if fields != "" {
				parts := strings.Split(line, delim)
				if len(parts) == 1 && onlyDelimited {
					continue
				}
				var out []string
				if complement {
					for n := 1; n <= len(parts); n++ {
						if !inSpans(ranges, n) {
							out = append(out, parts[n-1])
						}
					}
				} else {
					for _, sp := range ranges {
						for n := sp.from; n <= sp.to; n++ {
							if n >= 1 && n <= len(parts) {
								out = append(out, parts[n-1])
							}
						}
					}
				}
				fmt.Fprintln(w, strings.Join(out, join))
				continue
			}
			// -c counts characters, -b counts bytes. A whole line is one
			// unit for the selection either way; only the unit differs.
			var units []string
			if bytesMode {
				for _, b := range []byte(line) {
					units = append(units, string(b))
				}
			} else {
				for _, r := range line {
					units = append(units, string(r))
				}
			}
			if complement {
				var sb strings.Builder
				for n := 1; n <= len(units); n++ {
					if !inSpans(ranges, n) {
						sb.WriteString(units[n-1])
					}
				}
				fmt.Fprintln(w, sb.String())
				continue
			}
			var chunks []string
			for _, sp := range ranges {
				var sb strings.Builder
				for n := sp.from; n <= sp.to; n++ {
					if n >= 1 && n <= len(units) {
						sb.WriteString(units[n-1])
					}
				}
				if outDelim != "" && sb.Len() > 0 {
					chunks = append(chunks, sb.String())
				} else {
					chunks = append(chunks, sb.String())
				}
			}
			fmt.Fprintln(w, strings.Join(chunks, outDelim))
		}
	}
	return 0
}

type span struct {
	from int
	to   int
}

// parseList parses cut-style lists: "1", "1,3", "2-4", "5-".
func parseList(list string) ([]span, error) {
	var spans []span
	for _, part := range strings.Split(list, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i := strings.Index(part, "-"); i > 0 || (i == 0 && len(part) > 1) {
			from, err := strconv.Atoi(part[:i])
			if err != nil {
				return nil, fmt.Errorf("bad list %q", list)
			}
			to := 0
			if part[i+1:] == "" {
				to = 1 << 30
			} else {
				to, err = strconv.Atoi(part[i+1:])
				if err != nil {
					return nil, fmt.Errorf("bad list %q", list)
				}
			}
			if to < from {
				return nil, fmt.Errorf("decreasing range %q", part)
			}
			spans = append(spans, span{from, to})
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("bad list %q", list)
		}
		spans = append(spans, span{n, n})
	}
	if len(spans) == 0 {
		return nil, fmt.Errorf("empty list")
	}
	return spans, nil
}

// Helpers shared by several commands.
