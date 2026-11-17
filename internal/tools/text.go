package tools

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// runBasename implements basename [-a] [-s SUFFIX] NAME...
func runBasename(args []string, stdin *os.File, stdout, stderr *os.File) int {
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

// runDirname implements dirname NAME...
func runDirname(args []string, stdin *os.File, stdout, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "dirname: missing operand")
		return 2
	}
	for _, name := range args {
		clean := strings.TrimRight(name, "/\\")
		if clean == "" {
			fmt.Fprintln(stdout, ".")
			continue
		}
		i := strings.LastIndexAny(clean, "/\\")
		if i < 0 {
			fmt.Fprintln(stdout, ".")
			continue
		}
		dir := clean[:i]
		if dir == "" {
			dir = string([]rune{'/'})
		}
		fmt.Fprintln(stdout, dir)
	}
	return 0
}

// runNl implements nl [-b STYLE] [-n FORMAT] [-w WIDTH] [FILE]
func runNl(args []string, stdin *os.File, stdout, stderr *os.File) int {
	body := "t"    // t: number non-empty lines, a: number all
	format := "rn" // ln, rn, rz
	width := 6
	sep := "\t"
	var files []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-b":
			if i+1 >= len(args) {
				return 2
			}
			i++
			body = args[i]
		case "-n":
			if i+1 >= len(args) {
				return 2
			}
			i++
			format = args[i]
		case "-w":
			if i+1 >= len(args) {
				return 2
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				fmt.Fprintln(stderr, "nl: bad width")
				return 2
			}
			width = n
		case "-s":
			if i+1 >= len(args) {
				return 2
			}
			i++
			sep = args[i]
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				fmt.Fprintf(stderr, "nl: unknown option %q\n", args[i])
				return 2
			}
			files = append(files, args[i])
		}
	}
	readers, code := openInputs(files, stdin, stderr, "nl")
	if code != 0 {
		return code
	}
	defer closeInputs(readers)
	num := 0
	w := bufio.NewWriter(stdout)
	defer w.Flush()
	for _, r := range readers {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for sc.Scan() {
			line := sc.Text()
			numbered := body == "a" || (body == "t" && strings.TrimSpace(line) != "")
			if numbered {
				num++
				fmt.Fprint(w, formatNumber(num, format, width))
				fmt.Fprint(w, sep)
			} else {
				fmt.Fprint(w, strings.Repeat(" ", width))
			}
			fmt.Fprintln(w, line)
		}
	}
	return 0
}

func formatNumber(n int, format string, width int) string {
	s := strconv.Itoa(n)
	switch format {
	case "ln":
		return padRight(s, width)
	case "rz":
		return padLeft(strings.ReplaceAll(s, " ", "0"), width, '0')
	default: // rn
		return padLeft(s, width, ' ')
	}
}

func padLeft(s string, width int, fill rune) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(string(fill), width-len(s)) + s
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// runTac implements tac [FILE...]: print lines in reverse order.
func runTac(args []string, stdin *os.File, stdout, stderr *os.File) int {
	readers, code := openInputs(args, stdin, stderr, "tac")
	if code != 0 {
		return code
	}
	defer closeInputs(readers)
	w := bufio.NewWriter(stdout)
	defer w.Flush()
	for _, r := range readers {
		lines, err := readAllLines(r)
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
func runCut(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var delim string
	var fields, chars string
	onlyDelimited := false
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
		case strings.HasPrefix(a, "--delimiter"):
			a = "-d"
		case strings.HasPrefix(a, "--fields"):
			a = "-f"
		case strings.HasPrefix(a, "--characters"):
			a = "-c"
		case len(a) > 2 && (a[:2] == "-d" || a[:2] == "-f" || a[:2] == "-c"):
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
		case "-c":
			v, ok := getValue(&i, attached)
			if !ok {
				return 2
			}
			chars = v
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

	readers, code := openInputs(files, stdin, stderr, "cut")
	if code != 0 {
		return code
	}
	defer closeInputs(readers)
	w := bufio.NewWriter(stdout)
	defer w.Flush()
	for _, r := range readers {
		lines, err := readAllLines(r)
		if err != nil {
			fmt.Fprintf(stderr, "cut: %v\n", err)
			return 1
		}
		for _, line := range lines {
			if fields != "" {
				parts := strings.Split(line, delim)
				if len(parts) == 1 && onlyDelimited {
					continue
				}
				var out []string
				for _, sp := range ranges {
					for n := sp.from; n <= sp.to; n++ {
						if n >= 1 && n <= len(parts) {
							out = append(out, parts[n-1])
						}
					}
				}
				fmt.Fprintln(w, strings.Join(out, delim))
				continue
			}
			runes := []rune(line)
			var sb strings.Builder
			for _, sp := range ranges {
				for n := sp.from; n <= sp.to; n++ {
					if n >= 1 && n <= len(runes) {
						sb.WriteRune(runes[n-1])
					}
				}
			}
			fmt.Fprintln(w, sb.String())
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

func openInputs(names []string, stdin *os.File, stderr *os.File, cmd string) ([]io.Reader, int) {
	if len(names) == 0 {
		return []io.Reader{stdin}, 0
	}
	var readers []io.Reader
	for _, name := range names {
		if name == "-" {
			readers = append(readers, stdin)
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
			return nil, 1
		}
		readers = append(readers, f)
	}
	return readers, 0
}

func closeInputs(readers []io.Reader) {
	for _, r := range readers {
		if c, ok := r.(io.Closer); ok && c != os.Stdin {
			c.Close()
		}
	}
}

func readAllLines(r io.Reader) ([]string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	text := string(data)
	if text == "" {
		return nil, nil
	}
	text = strings.TrimSuffix(text, "\n")
	text = strings.TrimSuffix(text, "\r")
	return strings.Split(text, "\n"), nil
}
