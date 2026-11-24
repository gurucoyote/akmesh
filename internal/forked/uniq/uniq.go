// Copyright 2013-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/uniq/uniq.go.
//
// Local changes in akmesh (acmesh-6bq), all measured against GNU coreutils 9.7:
//
//   - -c prefixes the count in GNU's shape: right aligned in seven columns and
//     one space ("      1 b"), not a tab. u-root wrote "1\tb".
//   - -f N, -s N and -w N were missing; GNU has them, agents use them to group
//     log lines and fixed-width records.
//   - -z reads and writes NUL separated records, the pair to find -print0.
//   - The long spellings (--count, --repeated, --unique, --ignore-case,
//     --skip-fields, --skip-chars, --check-chars, --zero-terminated) exist.
//   - GNU's operands are INPUT [OUTPUT], not a list of inputs: a second operand
//     names the file the result is written to. u-root opened every operand as an
//     input. An unterminated last record counts, and an unknown option ends with
//     status 2 and a message like GNU's.
//
// The comparison itself keeps GNU's rule: adjacent records only, and the
// skipped prefix (-f, then -s) is cut off before the -w limit applies, while
// the printed line stays whole.

// Uniq removes repeated lines.
//
// Synopsis:
//
//	uniq [OPTIONS...] [FILES]...
//
// Description:
//
//	Uniq copies the input file, or the standard input, to the standard
//	output, comparing adjacent lines. In the normal case, the second and
//	succeeding copies of repeated lines are removed. Repeated lines must be
//	adjacent in order to be found.
//
// Options:
//
//	-c, --count              prefix a repetition count (seven columns, one space)
//	-d, --repeated           print only lines that repeat
//	-u, --unique             print only lines that do not repeat
//	-i, --ignore-case        compare case insensitively
//	-f N, --skip-fields=N    ignore the first N blank separated fields
//	-s N, --skip-chars=N     ignore the first N characters (after -f)
//	-w N, --check-chars=N    compare at most N characters (after -f and -s)
//	-z, --zero-terminated    separate records with NUL instead of a line break
package uniq

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// params holds the parsed command line.
type params struct {
	count, repeated, unique, ignoreCase bool
	zeroTerminated                      bool
	skipFields, skipChars, checkChars   int
	checkCharsSet                       bool
	files                               []string
}

// parse reads the arguments the way GNU's getopt would.
func parse(args []string) (*params, error) {
	p := &params{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			p.files = append(p.files, args[i+1:]...)
			return p, nil
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := strings.Cut(a[2:], "=")
			num := func() (int, error) {
				if !hasVal {
					if i+1 >= len(args) {
						return 0, fmt.Errorf("option '--%s' requires an argument", name)
					}
					i++
					val = args[i]
				}
				n, err := strconv.Atoi(val)
				if err != nil || n < 0 {
					return 0, fmt.Errorf("invalid number of fields to skip: %q", val)
				}
				return n, nil
			}
			var err error
			switch name {
			case "count":
				p.count = true
			case "repeated":
				p.repeated = true
			case "unique":
				p.unique = true
			case "ignore-case":
				p.ignoreCase = true
			case "zero-terminated":
				p.zeroTerminated = true
			case "skip-fields":
				p.skipFields, err = num()
			case "skip-chars":
				p.skipChars, err = num()
			case "check-chars":
				p.checkChars, err = num()
				p.checkCharsSet = true
			default:
				return nil, fmt.Errorf("unrecognized option '--%s'", name)
			}
			if err != nil {
				return nil, err
			}
		case len(a) > 1 && a[0] == '-':
			// Short options, glued or not: -ci, -c -i, -f2, -f 2.
			for j := 1; j < len(a); j++ {
				switch a[j] {
				case 'c':
					p.count = true
				case 'd':
					p.repeated = true
				case 'u':
					p.unique = true
				case 'i':
					p.ignoreCase = true
				case 'z':
					p.zeroTerminated = true
				case 'f', 's', 'w':
					v := a[j+1:]
					if v == "" {
						if i+1 >= len(args) {
							return nil, fmt.Errorf("option requires an argument -- '%c'", a[j])
						}
						i++
						v = args[i]
					}
					n, err := strconv.Atoi(v)
					if err != nil || n < 0 {
						return nil, fmt.Errorf("invalid number: %q", v)
					}
					switch a[j] {
					case 'f':
						p.skipFields = n
					case 's':
						p.skipChars = n
					case 'w':
						p.checkChars = n
						p.checkCharsSet = true
					}
					j = len(a)
				default:
					return nil, fmt.Errorf("invalid option -- '%c'", a[j])
				}
			}
		default:
			p.files = append(p.files, a)
		}
	}
	return p, nil
}

// key cuts the compared part out of a record: first the blank separated
// fields -f, then the characters -s, then the -w limit. GNU skips blanks and
// non-blanks together for every field.
func (p *params) key(line string) string {
	s := line
	for n := p.skipFields; n > 0 && s != ""; n-- {
		for s != "" && (s[0] == ' ' || s[0] == '\t') {
			s = s[1:]
		}
		for s != "" && s[0] != ' ' && s[0] != '\t' {
			s = s[1:]
		}
	}
	if p.skipChars > 0 {
		if p.skipChars >= len(s) {
			s = ""
		} else {
			s = s[p.skipChars:]
		}
	}
	if p.checkCharsSet && p.checkChars < len(s) {
		s = s[:p.checkChars]
	}
	if p.ignoreCase {
		s = strings.ToLower(s)
	}
	return s
}

// records splits the input into records on the separator and keeps an
// unterminated last record, as GNU does.
func records(r io.Reader, sep byte) ([]string, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	var out []string
	var cur strings.Builder
	for {
		b, err := br.ReadByte()
		if err == io.EOF {
			if cur.Len() > 0 {
				out = append(out, cur.String())
			}
			return out, nil
		}
		if err != nil {
			return out, err
		}
		if b == sep {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(b)
	}
}

// run implements uniq over one reader.
func run(p *params, in io.Reader, out io.Writer) error {
	sep := byte('\n')
	if p.zeroTerminated {
		sep = 0
	}
	lines, err := records(in, sep)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(out)
	defer w.Flush()
	emit := func(line string, cnt int) {
		if p.count {
			fmt.Fprintf(w, "%7d %s", cnt, line)
		} else {
			fmt.Fprintf(w, "%s", line)
		}
		w.WriteByte(sep)
	}
	if len(lines) == 0 {
		return nil
	}
	prev := lines[0]
	prevKey := p.key(prev)
	cnt := 1
	for _, line := range lines[1:] {
		if k := p.key(line); k == prevKey {
			cnt++
			continue
		}
		p.maybeEmit(emit, prev, cnt)
		prev, prevKey, cnt = line, p.key(line), 1
	}
	p.maybeEmit(emit, prev, cnt)
	return nil
}

// maybeEmit applies -u and -d: plain output prints every group once, -d only
// groups that repeat, -u only groups that do not, and both together nothing.
func (p *params) maybeEmit(emit func(string, int), line string, cnt int) {
	switch {
	case p.repeated && p.unique:
		return
	case p.repeated && cnt < 2:
		return
	case p.unique && cnt > 1:
		return
	}
	emit(line, cnt)
}

// Run is the entry point for the dispatcher: it returns the exit status.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	p, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "uniq: %v\n", err)
		fmt.Fprintf(stderr, "Try 'uniq --help' for more information.\n")
		return 2
	}
	if len(p.files) > 2 {
		fmt.Fprintf(stderr, "uniq: extra operand %q\n", p.files[2])
		fmt.Fprintf(stderr, "Try 'uniq --help' for more information.\n")
		return 1
	}
	// GNU's operands are INPUT [OUTPUT] (POSIX), not a list of inputs.
	in := stdin
	if len(p.files) > 0 && p.files[0] != "-" {
		f, err := os.Open(p.files[0])
		if err != nil {
			fmt.Fprintf(stderr, "uniq: %s\n", errText(err))
			return 1
		}
		defer f.Close()
		in = f
	}
	out := stdout
	if len(p.files) > 1 {
		f, err := os.Create(p.files[1])
		if err != nil {
			fmt.Fprintf(stderr, "uniq: %s\n", errText(err))
			return 1
		}
		defer f.Close()
		out = f
	}
	if err := run(p, in, out); err != nil {
		fmt.Fprintf(stderr, "uniq: %v\n", err)
		return 1
	}
	return 0
}

// errText is GNU's wording for a file that cannot be opened.
func errText(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return fmt.Sprintf("%s: %v", pe.Path, pe.Err)
	}
	return err.Error()
}
