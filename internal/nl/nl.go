// Package nl implements nl, numbering the lines of a file, the GNU way.
// nl numbers lines the way GNU nl does.
//
// This is akmesh's own implementation, not a fork: the u-root command takes
// only a handful of flags and fails on -v, -i, -p, -l, -h and -f, which an
// agent that knows GNU will use (measured 24.11.2026, acmesh-efg).
//
// Numbered lines get the number right aligned in -w columns (or left aligned
// with -n ln, zero padded with -n rz) followed by the separator -s. Lines that
// are not numbered are indented by one column more than the width, so the text
// lines up, as GNU does.
package nl

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type options struct {
	body, header, footer byte // 'a' all, 't' non-empty, 'n' none, 'p' regex
	bodyRe               *regexp.Regexp
	headerRe             *regexp.Regexp
	footerRe             *regexp.Regexp
	delim                string
	joinBlank            int // -l: blank lines counted as one
	numberFormat         string
	width                int
	sep                  string
	start                int64
	increment            int64
}

func defaults() options {
	return options{
		body: 't', header: 'n', footer: 'n',
		delim: `\:`, joinBlank: 1, numberFormat: "rn",
		width: 6, sep: "\t", start: 1, increment: 1,
	}
}

// formatNumber renders one number in the chosen style.
func (o options) formatNumber(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) >= o.width {
		return s
	}
	switch o.numberFormat {
	case "ln":
		return s + strings.Repeat(" ", o.width-len(s))
	case "rz":
		return strings.Repeat("0", o.width-len(s)) + s
	default:
		return strings.Repeat(" ", o.width-len(s)) + s
	}
}

// matches reports whether a line of this section takes a number.
func (o options) matches(section byte, line string) bool {
	mode, re := o.body, o.bodyRe
	switch section {
	case 'H':
		mode, re = o.header, o.headerRe
	case 'F':
		mode, re = o.footer, o.footerRe
	}
	switch mode {
	case 'a':
		return true
	case 'n':
		return false
	case 'p':
		return re != nil && re.MatchString(line)
	default:
		return line != ""
	}
}

// numberMode reads the a/t/n/pRE value of -b, -h or -f.
func numberMode(v string, o *options, which byte) error {
	switch {
	case v == "a" || v == "t" || v == "n":
		mode := v[0]
		switch which {
		case 'b':
			o.body = mode
		case 'h':
			o.header = mode
		case 'f':
			o.footer = mode
		}
		return nil
	case strings.HasPrefix(v, "p"):
		re, err := regexp.Compile(v[1:])
		if err != nil {
			return fmt.Errorf("invalid regular expression: %s", v[1:])
		}
		switch which {
		case 'b':
			o.body, o.bodyRe = 'p', re
		case 'h':
			o.header, o.headerRe = 'p', re
		case 'f':
			o.footer, o.footerRe = 'p', re
		}
		return nil
	}
	switch which {
	case 'b':
		return fmt.Errorf("invalid body numbering style: '%s'", v)
	case 'h':
		return fmt.Errorf("invalid header numbering style: '%s'", v)
	}
	return fmt.Errorf("invalid footer numbering style: '%s'", v)
}

// Run numbers stdin or the given files.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	o := defaults()
	var files []string
	take := func(i *int, glued, opt string) (string, error) {
		if glued != "" {
			return glued, nil
		}
		*i++
		if *i >= len(args) {
			return "", fmt.Errorf("option requires an argument -- '%s'", opt)
		}
		return args[*i], nil
	}
	bad := func(err error) int {
		fmt.Fprintf(stderr, "nl: %v\n", err)
		fmt.Fprintf(stderr, "Try 'nl --help' for more information.\n")
		return 1
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case a == "--help":
			usage(stdout)
			return 0
		case len(a) > 1 && a[0] == '-' && a != "-":
			rest := a[1:]
			for rest != "" {
				c := rest[0]
				rest = rest[1:]
				switch c {
				case 'p':
					// Told apart from -b pREGEX below; -p alone means "no
					// renumbering over page breaks", which never happens here.
					continue
				case 'b', 'h', 'f', 'd', 'l', 'w', 's', 'n', 'v', 'i':
					v, err := take(&i, rest, string(c))
					if err != nil {
						return bad(err)
					}
					rest = ""
					switch c {
					case 'b', 'h', 'f':
						if err := numberMode(v, &o, c); err != nil {
							return bad(err)
						}
					case 'd':
						o.delim = v
					case 'l':
						n, err := strconv.Atoi(v)
						if err != nil || n < 1 {
							return bad(fmt.Errorf("invalid line number of blank lines: '%s'", v))
						}
						o.joinBlank = n
					case 'w':
						n, err := strconv.Atoi(v)
						if err != nil || n < 1 {
							return bad(fmt.Errorf("invalid line number field width: '%s'", v))
						}
						o.width = n
					case 's':
						o.sep = v
					case 'n':
						switch v {
						case "ln", "rn", "rz":
							o.numberFormat = v
						default:
							return bad(fmt.Errorf("invalid line numbering format: '%s'", v))
						}
					case 'v':
						n, err := strconv.ParseInt(v, 10, 64)
						if err != nil {
							return bad(fmt.Errorf("invalid starting line number: '%s'", v))
						}
						o.start = n
					case 'i':
						n, err := strconv.ParseInt(v, 10, 64)
						if err != nil {
							return bad(fmt.Errorf("invalid line number increment: '%s'", v))
						}
						o.increment = n
					}
				default:
					return bad(fmt.Errorf("invalid option -- '%c'", c))
				}
			}
		default:
			files = append(files, a)
		}
	}

	if len(files) == 0 {
		return number(o, stdin, stdout, stderr)
	}
	status := 0
	for _, name := range files {
		var r io.Reader = stdin
		var f io.Closer
		if name != "-" {
			file, err := openFile(name)
			if err != nil {
				fmt.Fprintf(stderr, "nl: %s: %s\n", name, sentence(err))
				status = 1
				continue
			}
			r, f = file, file
		}
		if rc := number(o, r, stdout, stderr); rc != 0 {
			status = rc
		}
		if f != nil {
			f.Close()
		}
	}
	return status
}

// number writes the numbered stream. Sections switch at a line that is exactly
// the delimiter (header), twice (body) and three times (footer).
func number(o options, in io.Reader, out io.Writer, stderr io.Writer) int {
	r := bufio.NewReader(in)
	w := bufio.NewWriter(out)
	defer w.Flush()
	section := byte('B')
	delimCount := 0
	next := o.start
	blankRun := 0
	for {
		line, err := r.ReadString('\n')
		if line != "" {
			trimmed := strings.TrimSuffix(line, "\n")
			if trimmed == o.delim {
				delimCount++
				switch delimCount {
				case 1:
					section = 'H'
				case 2:
					section = 'B'
				case 3:
					section = 'F'
				}
				// A delimiter line prints as an empty line, as GNU does.
				fmt.Fprint(w, "\n")
				if err != nil {
					break
				}
				continue
			}
			if trimmed == "" {
				blankRun++
			} else {
				blankRun = 0
			}
			// A group of blank lines takes one number per -l lines.
			if trimmed == "" && blankRun > 1 && blankRun%o.joinBlank != 0 {
				fmt.Fprint(w, strings.Repeat(" ", o.width+1)+line)
				if err != nil {
					break
				}
				continue
			}
			if o.matches(section, trimmed) {
				fmt.Fprint(w, o.formatNumber(next)+o.sep+line)
				next += o.increment
			} else {
				fmt.Fprint(w, strings.Repeat(" ", o.width+1)+line)
			}
		}
		if err != nil {
			break
		}
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprint(w, `Usage: nl [OPTION]... [FILE]...
Write each FILE to standard output, with line numbers added.

  -b STYLE   number body lines: a all, t non-empty (default), n none, pREGEX
  -h STYLE   number header lines (default n)
  -f STYLE   number footer lines (default n)
  -d CC      section delimiters (default \:)
  -l N       count N blank lines as one (default 1)
  -w N       number width (default 6)
  -s STRING  separator after the number (default TAB)
  -n FORMAT  ln left, rn right (default), rz right with leading zeros
  -v N       first number (default 1)
  -i N       increment (default 1)
`)
}
