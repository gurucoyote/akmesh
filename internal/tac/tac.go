// Package tac implements tac, printing records in reverse, the GNU way.
// tac prints the input backwards, record by record, the way GNU tac does.
//
// akmesh's own implementation: the u-root command has no -b and no -r, and its
// message on a missing file is "tac: open x: no such file or directory"
// instead of GNU's "tac: failed to open 'x' for reading: ..." (measured
// 24.11.2026, acmesh-efg).
//
// The separator belongs to the end of a record by default and to its start
// with -b; that is the only difference between the two modes, and it decides
// where a trailing separator ends up.
package tac

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

type options struct {
	sep    string // the separator text, after escapes have been read
	sepRe  *regexp.Regexp
	regex  bool
	before bool
}

// unescape reads the escape sequences GNU understands in -s: \n, \t, \0, \r,
// \\ and an octal \NNN.
func unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch c := s[i]; c {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '\\':
			b.WriteByte('\\')
		case '0', '1', '2', '3', '4', '5', '6', '7':
			v := int(c - '0')
			for k := 0; k < 2 && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7'; k++ {
				i++
				v = v*8 + int(s[i]-'0')
			}
			b.WriteByte(byte(v))
		default:
			b.WriteByte('\\')
			b.WriteByte(c)
		}
	}
	return b.String()
}

// splitRecords cuts the text into parts and the separators between them.
// seps[i] is the text that ended parts[i], so a record can carry its own
// separator - with -r the match itself, not the pattern (found 24.11.2026:
// the pattern text was written out as the separator).
func splitRecords(o options, text string) (parts, seps []string) {
	if o.regex {
		last := 0
		for _, m := range o.sepRe.FindAllStringIndex(text, -1) {
			parts = append(parts, text[last:m[0]])
			seps = append(seps, text[m[0]:m[1]])
			last = m[1]
		}
		return append(parts, text[last:]), seps
	}
	parts = strings.Split(text, o.sep)
	for i := 0; i < len(parts)-1; i++ {
		seps = append(seps, o.sep)
	}
	return parts, seps
}

// reverse turns the whole input around, record by record.
func reverse(o options, data []byte) string {
	text := string(data)
	if !o.regex && o.sep == "" {
		// An empty separator makes the whole input one record, so printing the
		// records in reverse leaves it as it was (measured against GNU tac).
		return text
	}
	parts, seps := splitRecords(o, text)
	var records []string
	if o.before {
		// The separator starts the record: the first part has none, every
		// later one carries its own separator at the front.
		for i, p := range parts {
			if i == 0 {
				records = append(records, p)
				continue
			}
			records = append(records, seps[i-1]+p)
		}
	} else {
		// The separator ends the record; the empty part after a final
		// separator is no record of its own.
		for i, p := range parts {
			if i == len(parts)-1 && p == "" && len(seps) == len(parts)-1 {
				continue
			}
			r := p
			if i < len(seps) {
				r += seps[i]
			}
			records = append(records, r)
		}
	}
	var b strings.Builder
	for i := len(records) - 1; i >= 0; i-- {
		b.WriteString(records[i])
	}
	return b.String()
}

// Run is the command.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	o := options{sep: "\n"}
	var files []string
	bad := func(err error) int {
		fmt.Fprintf(stderr, "tac: %v\n", err)
		fmt.Fprintf(stderr, "Try 'tac --help' for more information.\n")
		return 1
	}
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
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case a == "--help":
			fmt.Fprint(stdout, `Usage: tac [OPTION]... [FILE]...
Write each FILE to standard output, last line first.

  -b, --before            attach the separator before instead of after
  -r, --regex             read the separator as a regular expression
  -s, --separator=STRING  use STRING as the separator (default newline)
`)
			return 0
		case a == "-b" || a == "--before":
			o.before = true
		case a == "-r" || a == "--regex":
			o.regex = true
		case a == "-s" || a == "--separator" || strings.HasPrefix(a, "--separator=") || strings.HasPrefix(a, "-s"):
			glued := ""
			switch {
			case strings.HasPrefix(a, "--separator="):
				glued = strings.TrimPrefix(a, "--separator=")
			case a != "-s":
				glued = a[2:]
			}
			v, err := take(&i, glued, "s")
			if err != nil {
				return bad(err)
			}
			o.sep = unescape(v)
		case len(a) > 1 && a[0] == '-' && a != "-":
			// Short flags cluster: -bs: is -b -s :.
			rest := a[1:]
			for rest != "" {
				c := rest[0]
				rest = rest[1:]
				switch c {
				case 'b':
					o.before = true
				case 'r':
					o.regex = true
				case 's':
					v, err := take(&i, rest, "s")
					if err != nil {
						return bad(err)
					}
					o.sep = unescape(v)
					rest = ""
				default:
					return bad(fmt.Errorf("invalid option -- '%c'", c))
				}
			}
		default:
			files = append(files, a)
		}
	}
	if o.regex {
		re, err := regexp.Compile(o.sep)
		if err != nil {
			return bad(fmt.Errorf("invalid regex: %s", o.sep))
		}
		o.sepRe = re
	}

	if len(files) == 0 {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return bad(err)
		}
		fmt.Fprint(stdout, reverse(o, data))
		return 0
	}
	status := 0
	w := bufio.NewWriter(stdout)
	defer w.Flush()
	for _, name := range files {
		var data []byte
		var err error
		if name == "-" {
			data, err = io.ReadAll(stdin)
		} else {
			data, err = os.ReadFile(name)
		}
		if err != nil {
			fmt.Fprintf(stderr, "tac: failed to open '%s' for reading: %s\n", name, sentence(err))
			status = 1
			continue
		}
		w.WriteString(reverse(o, data))
	}
	return status
}

// sentence is the system's reason with glibc's capital letter.
func sentence(err error) string {
	var pe *os.PathError
	if errors.As(err, &pe) {
		err = pe.Err
	}
	msg := err.Error()
	if msg != "" && msg[0] >= 'a' && msg[0] <= 'z' {
		msg = string(msg[0]-32) + msg[1:]
	}
	return msg
}
