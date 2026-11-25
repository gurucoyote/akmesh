// Package cut implements cut, selecting fields, bytes and characters.

package cut

import (
	"akmesh/internal/shared"
	akusage "akmesh/internal/usage"
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// badUsage prints the message in GNU's shape and returns its exit status. cut
// uses 1 for a bad command line, not 2.
func badUsage(stderr *os.File, msg string) int {
	fmt.Fprintf(stderr, "cut: %s\n", msg)
	fmt.Fprintln(stderr, "Try 'cut --help' for more information.")
	return 1
}

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
				return badUsage(stderr, "the delimiter must be a single character")
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
			if chars != "" {
				// GNU refuses two kinds of list instead of letting the last win.
				return badUsage(stderr, "only one list may be specified")
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
				return akusage.BadOption("cut", args[i], stderr, 1)
			}
			files = append(files, args[i])
		}
	}
	if (fields == "") == (chars == "") {
		return badUsage(stderr, "you must specify a list of bytes, characters, or fields")
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
				if len(parts) == 1 {
					if onlyDelimited {
						continue
					}
					// A line without the delimiter is printed whole, as GNU
					// does, instead of leaving an empty line behind.
					fmt.Fprintln(w, line)
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
			// -c counts characters, -b counts bytes. A byte slice keeps the
			// bytes as they are: turning them into one-rune strings and back
			// would re-encode each of them as UTF-8 (two bytes became four).
			if bytesMode {
				b := []byte(line)
				out := make([]byte, 0, len(b))
				for n := 1; n <= len(b); n++ {
					if inSpans(ranges, n) != complement {
						out = append(out, b[n-1])
					}
				}
				w.Write(out)
				w.WriteByte('\n')
				continue
			}
			runes := []rune(line)
			var chunks []string
			for n := 1; n <= len(runes); n++ {
				if inSpans(ranges, n) != complement {
					chunks = append(chunks, string(runes[n-1]))
				}
			}
			if complement || len(ranges) > 1 {
				fmt.Fprintln(w, strings.Join(chunks, outDelim))
				continue
			}
			fmt.Fprintln(w, strings.Join(chunks, ""))
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
	const open = 1 << 30
	for part := range strings.SplitSeq(list, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if i := strings.Index(part, "-"); i >= 0 {
			// "-2" means one to two, GNU's open start.
			from := 1
			if i > 0 {
				n, err := strconv.Atoi(part[:i])
				if err != nil || n < 1 {
					return nil, fmt.Errorf("bad list %q", list)
				}
				from = n
			}
			to := open
			if part[i+1:] != "" {
				n, err := strconv.Atoi(part[i+1:])
				if err != nil || n < 1 {
					return nil, fmt.Errorf("bad list %q", list)
				}
				to = n
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
	// GNU prints the selection in increasing order and each field once, so
	// "3,1" gives "a c" and "1-2,2-3" is the same as "1-3".
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].from != spans[j].from {
			return spans[i].from < spans[j].from
		}
		return spans[i].to < spans[j].to
	})
	merged := []span{spans[0]}
	for _, sp := range spans[1:] {
		last := &merged[len(merged)-1]
		if sp.from <= last.to+1 {
			if sp.to > last.to {
				last.to = sp.to
			}
			continue
		}
		merged = append(merged, sp)
	}
	return merged, nil
}
