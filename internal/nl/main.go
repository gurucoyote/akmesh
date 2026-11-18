// Package nl implements nl, numbering the lines of a file.

package nl

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"acmesh/internal/shared"
)

// Run implements nl [-b STYLE] [-n FORMAT] [-w WIDTH] [FILE]
// nlValue returns an option's value, either attached (-ba) or the next word.
func nlValue(args []string, i *int, attached string) (string, bool) {
	if attached != "" {
		return attached, true
	}
	if *i+1 >= len(args) {
		return "", false
	}
	*i++
	return args[*i], true
}

func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	body := "t"    // t: number non-empty lines, a: number all
	format := "rn" // ln, rn, rz
	width := 6
	sep := "\t"
	delim := "\\:"
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		attached := ""
		switch {
		case a == "-b" || a == "-n" || a == "-w" || a == "-s" || a == "-d":
		case strings.HasPrefix(a, "--body-numbering="):
			a, attached = "-b", strings.TrimPrefix(a, "--body-numbering=")
		case strings.HasPrefix(a, "--number-format="):
			a, attached = "-n", strings.TrimPrefix(a, "--number-format=")
		case strings.HasPrefix(a, "--number-width="):
			a, attached = "-w", strings.TrimPrefix(a, "--number-width=")
		case strings.HasPrefix(a, "--number-separator="):
			a, attached = "-s", strings.TrimPrefix(a, "--number-separator=")
		case strings.HasPrefix(a, "--section-delimiter="):
			a, attached = "-d", strings.TrimPrefix(a, "--section-delimiter=")
		case len(a) > 2 && a[0] == '-' && strings.ContainsRune("bnwsd", rune(a[1])):
			// combined form such as -ba or -s::
			a, attached = a[:2], a[2:]
		case a == "--body-numbering":
			a = "-b"
		case a == "--number-format":
			a = "-n"
		case a == "--number-width":
			a = "-w"
		case a == "--number-separator":
			a = "-s"
		case a == "--section-delimiter":
			a = "-d"
		}
		args[i] = a
		switch args[i] {
		case "-b":
			v, ok := nlValue(args, &i, attached)
			if !ok {
				fmt.Fprintln(stderr, "nl: option -b needs an argument")
				return 2
			}
			body = v
		case "-n":
			v, ok := nlValue(args, &i, attached)
			if !ok {
				fmt.Fprintln(stderr, "nl: option -n needs an argument")
				return 2
			}
			format = v
		case "-w":
			v, ok := nlValue(args, &i, attached)
			if !ok {
				fmt.Fprintln(stderr, "nl: option -w needs an argument")
				return 2
			}
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				fmt.Fprintln(stderr, "nl: bad width")
				return 2
			}
			width = n
		case "-s":
			v, ok := nlValue(args, &i, attached)
			if !ok {
				fmt.Fprintln(stderr, "nl: option -s needs an argument")
				return 2
			}
			sep = v
		case "-d":
			v, ok := nlValue(args, &i, attached)
			if !ok {
				fmt.Fprintln(stderr, "nl: option -d needs an argument")
				return 2
			}
			delim = strings.TrimSpace(v)
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				fmt.Fprintf(stderr, "nl: unknown option %q\n", args[i])
				return 2
			}
			files = append(files, args[i])
		}
	}
	readers, code := shared.OpenInputs(files, stdin, stderr, "nl")
	if code != 0 {
		return code
	}
	defer shared.CloseInputs(readers)
	num := 0
	w := bufio.NewWriter(stdout)
	defer w.Flush()
	// Logical pages: a delimiter line ends a section. Only body lines are
	// numbered (GNU's default), and the body delimiter restarts the count.
	headerLine, bodyLine, footerLine := delim+delim+delim, delim+delim, delim
	inBody := true
	for _, r := range readers {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for sc.Scan() {
			line := sc.Text()
			switch line {
			case headerLine:
				fmt.Fprintln(w, line)
				inBody = false
				continue
			case footerLine:
				if line != bodyLine {
					fmt.Fprintln(w, line)
					inBody = false
					continue
				}
			}
			if line == bodyLine && bodyLine != footerLine {
				fmt.Fprintln(w, line)
				inBody, num = true, 0
				continue
			}
			numbered := inBody && (body == "a" || (body == "t" && strings.TrimSpace(line) != ""))
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
// tacWithSeparator reverses records instead of lines.
