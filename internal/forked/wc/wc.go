// Copyright 2013-2023 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Wc counts lines, words, runes, syntactically–invalid UTF codes.
//
// Synopsis:
//
//	wc [OPTIONS...] [FILES]...
//
// Description:
//
//	Wc counts lines, words, runes, syntactically–invalid UTF codes and bytes
//	in the named files, or in the standard input if no file is named. A word
//	is a maximal string of characters delimited by spaces, tabs or newlines.
//	The count of runes includes invalid codes. If the optional argument is
//	present, just the specified counts (lines, words, runes, broken UTF
//	codes or bytes) are selected by the letters l, w, r, b, or c. Otherwise,
//	lines, words and bytes (–lwc) are reported.
//
// Options:
//
//	–l: count lines
//	–w: count words
//	–r: count runes
//	–b: count broken UTF codes
//	-c: count bytes
//
// Bugs:
//
//	This wc differs from Plan 9's wc somewhat in word count (BSD's wc differs
//	even more significantly):
//
//	$ unicode 0x0-0x10ffff | 9 wc -w
//	2228221
//	$ unicode 0x0-0x10ffff | gowc -w
//	2228198
//	$ unicode 0x0-0x10ffff | bsdwc -w
//	2293628
//
//	This wc differs from Plan 9's wc significantly in bad rune count:
//
//	$ unicode 0x0-0x10ffff | gowc -b
//	6144
//	$ unicode 0x0-0x10ffff | 9 wc -b
//	1966080
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/wc/wc.go.
// The rest of this file is the upstream source and keeps its copyright.
//
// Local changes, all of them for the way agents call wc:
//   - package wc and Run(args, stdin, stdout, stderr) instead of main();
//   - no flag package: an own parser, so -lw and -m are read like GNU reads them;
//   - new: -m (characters, GNU's spelling; upstream only knew -r) and
//     --files0-from=FILE (read the file names NUL-separated, as find -print0
//     writes them);
//   - a file that cannot be opened is named on stderr and exits 1.
package wc

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// files0From reads a NUL-separated list of names, as "find -print0" writes it.
func files0From(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(string(data), "\x00")
	names := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			names = append(names, part)
		}
	}
	return names, nil
}

type params struct {
	lines  bool
	words  bool
	runes  bool
	broken bool
	chars  bool
}

type cmd struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	args   []string
	params
}

func command(stdin io.Reader, stdout io.Writer, stderr io.Writer, p params, args []string) *cmd {
	return &cmd{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		params: p,
		args:   args,
	}
}

func (c *cmd) run() error {
	var totals cnt
	if !c.lines && !c.words && !c.runes && !c.broken && !c.chars {
		c.lines, c.words, c.chars = true, true, true
	}

	if len(c.args) == 0 {
		res := c.count(c.stdin, "")
		c.report(res, "")
		return nil
	}

	var errs error

	for _, v := range c.args {
		f, err := os.Open(v)
		if err != nil {
			errs = errors.Join(errs, err)
			continue
		}
		res := c.count(f, v)
		totals.lines += res.lines
		totals.words += res.words
		totals.runes += res.runes
		totals.badRunes += res.badRunes
		totals.chars += res.chars
		c.report(res, v)
	}
	if len(c.args) > 1 {
		c.report(totals, "total")
	}
	return errs
}

type cnt struct {
	lines, words, runes, badRunes, chars int64
}

// A modified version of utf8.Valid()
func invalidCount(p []byte) (n int64) {
	i := 0
	for i < len(p) {
		if p[i] < utf8.RuneSelf {
			i++
		} else {
			_, size := utf8.DecodeRune(p[i:])
			if size == 1 {
				// All valid runes of size 1 (those
				// below RuneSelf) were handled above.
				// This muse be a RuneError.
				n++
			}
			i += size
		}
	}
	return
}

func (c *cmd) count(in io.Reader, fname string) cnt {
	b := bufio.NewReaderSize(in, 8192)
	counted := false
	count := cnt{}
	for !counted {
		line, err := b.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				counted = true
			} else {
				fmt.Fprintf(c.stderr, "wc: %s: %v\n", fname, err)
				return cnt{} // no partial counts; should perhaps quit altogether?
			}
		}
		if !counted {
			count.lines++
		}
		count.words += int64(len(bytes.Fields(line)))
		count.runes += int64(utf8.RuneCount(line))
		count.chars += int64(len(line))
		count.badRunes += invalidCount(line)
	}
	return count
}

func (c *cmd) report(count cnt, fname string) {
	fields := []string{}
	if c.lines {
		fields = append(fields, fmt.Sprintf("%d", count.lines))
	}
	if c.words {
		fields = append(fields, fmt.Sprintf("%d", count.words))
	}
	if c.runes {
		fields = append(fields, fmt.Sprintf("%d", count.runes))
	}
	if c.broken {
		fields = append(fields, fmt.Sprintf("%d", count.badRunes))
	}
	if c.chars {
		fields = append(fields, fmt.Sprintf("%d", count.chars))
	}
	if fname != "" {
		fields = append(fields, fname)
	}

	fmt.Fprintln(c.stdout, strings.Join(fields, " "))
}

// Run implements wc for the akmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var p params
	var rest []string
	files0 := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
		case a == "--lines":
			p.lines = true
		case a == "--words":
			p.words = true
		case a == "--chars":
			p.runes = true
		case a == "--bytes":
			p.chars = true
		case strings.HasPrefix(a, "--files0-from="):
			files0 = strings.TrimPrefix(a, "--files0-from=")
		case a == "--files0-from":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "wc: --files0-from needs a file")
				return 1
			}
			i++
			files0 = args[i]
		case strings.HasPrefix(a, "--"):
			// An unknown long option: name the whole token, not its first
			// letter, which used to print `unknown option "--"` (acmesh-vr1).
			fmt.Fprintf(stderr, "wc: unknown option %q\n", a)
			return 1
		case strings.HasPrefix(a, "-") && len(a) > 1:
			for j := 0; j < len(a[1:]); j++ {
				switch a[1+j] {
				case 'l':
					p.lines = true
				case 'w':
					p.words = true
				case 'm', 'r':
					// -m is GNU's spelling for characters; -r is u-root's.
					p.runes = true
				case 'c':
					p.chars = true
				case 'b':
					p.broken = true
				default:
					fmt.Fprintf(stderr, "wc: unknown option %q\n", "-"+string(a[1+j]))
					return 1
				}
			}
		default:
			rest = append(rest, a)
		}
	}
	if files0 != "" {
		names, err := files0From(files0)
		if err != nil {
			fmt.Fprintf(stderr, "wc: %v\n", err)
			return 1
		}
		rest = append(rest, names...)
	}
	if err := command(stdin, stdout, stderr, p, rest).run(); err != nil {
		fmt.Fprintf(stderr, "wc: %v\n", err)
		return 1
	}
	return 0
}
