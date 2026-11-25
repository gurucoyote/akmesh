// Copyright 2013-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/comm/comm.go.
//
// Local changes in akmesh (acmesh-y16), all measured against GNU coreutils 9.7:
//
//   - The flag package is gone, so the spellings GNU documents work: -12, -13,
//     -23, -123 as one argument, and the long forms --total, --check-order,
//     --nocheck-order and --output-delimiter=STR.
//   - Empty lines are printed. u-root dropped every record whose output would
//     have been two tabs, which is exactly an empty common line.
//   - A trailing tab is no longer trimmed: GNU keeps the tabs of the columns
//     before the printed one, so a common empty line is "\t\t".
//   - A suppressed column no longer indents the record: "comm -12" prints the
//     common lines without leading tabs, as GNU does.
//   - "-" means standard input, as in GNU.
//   - The two files are read in one loop instead of three goroutines; the
//     output order is the same and an error in one file no longer leaves
//     goroutines behind.

// comm compares two sorted files line by line.
//
// Synopsis:
//
//	comm [OPTION]... FILE1 FILE2
//
// Description:
//
//	Produce three tab separated columns: lines only in FILE1, lines only in
//	FILE2 and lines in both. A file name of - means standard input.
//
// Options:
//
//	-1              suppress column 1 (lines only in FILE1)
//	-2              suppress column 2 (lines only in FILE2)
//	-3              suppress column 3 (lines in both)
//	--output-delimiter=STR  separate columns with STR instead of a tab
//	--total         print a final line with the counts per column and the sum
//	--check-order   fail when a file is not sorted
//	--nocheck-order accept unsorted files (the default, as in GNU)
package comm

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// params is the parsed command line.
type params struct {
	sup        [3]bool
	total      bool
	checkOrder bool
	delim      string
	files      []string
}

// parse reads the arguments the way GNU's getopt would: glued short flags
// (-12, -123) are one word, long options may carry =VALUE.
func parse(args []string) (*params, error) {
	p := &params{delim: "\t"}
	for i := range args {
		a := args[i]
		switch {
		case a == "--":
			p.files = append(p.files, args[i+1:]...)
			return p, nil
		case a == "--help" || a == "-h":
			return nil, errHelp
		case strings.HasPrefix(a, "--"):
			name, val, hasVal := strings.Cut(a[2:], "=")
			switch name {
			case "total":
				p.total = true
			case "check-order":
				p.checkOrder = true
			case "nocheck-order":
				p.checkOrder = false
			case "output-delimiter":
				if !hasVal {
					return nil, fmt.Errorf("option '--output-delimiter' requires an argument")
				}
				p.delim = val
			default:
				return nil, fmt.Errorf("unrecognized option '--%s'", name)
			}
		case len(a) > 1 && a[0] == '-':
			if a == "-" {
				p.files = append(p.files, a)
				continue
			}
			for _, c := range a[1:] {
				switch c {
				case '1', '2', '3':
					p.sup[c-'1'] = true
				default:
					return nil, fmt.Errorf("invalid option -- '%c'", c)
				}
			}
		default:
			p.files = append(p.files, a)
		}
	}
	return p, nil
}

// errHelp asks the caller to print the usage text.
var errHelp = fmt.Errorf("help requested")

// lines splits the input on line breaks and keeps an unterminated last line,
// as GNU does.
func lines(r io.Reader) ([]string, error) {
	br := bufio.NewReaderSize(r, 64*1024)
	var out []string
	for {
		s, err := br.ReadString('\n')
		if s != "" {
			out = append(out, strings.TrimSuffix(strings.TrimSuffix(s, "\n"), "\r"))
		}
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return out, err
		}
	}
}

// sorted reports the first place where the list is out of order, or -1.
func sorted(list []string) int {
	for i := 1; i < len(list); i++ {
		if list[i-1] > list[i] {
			return i
		}
	}
	return -1
}

// Run is the entry point for the dispatcher: it returns the exit status.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	p, err := parse(args)
	if err != nil {
		if err == errHelp {
			return 0
		}
		fmt.Fprintf(stderr, "comm: %v\n", err)
		fmt.Fprintf(stderr, "Try 'comm --help' for more information.\n")
		return 1
	}
	if len(p.files) != 2 {
		fmt.Fprintf(stderr, "comm: missing operand\n")
		fmt.Fprintf(stderr, "Try 'comm --help' for more information.\n")
		return 1
	}
	var lists [2][]string
	for i, name := range p.files {
		var r io.Reader = stdin
		if name != "-" {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(stderr, "comm: %s\n", errText(err))
				return 1
			}
			defer f.Close()
			r = f
		}
		list, err := lines(r)
		if err != nil {
			fmt.Fprintf(stderr, "comm: %s: %v\n", name, err)
			return 1
		}
		if p.checkOrder {
			if at := sorted(list); at >= 0 {
				fmt.Fprintf(stderr, "comm: file %d is not in sorted order\n", i+1)
				return 1
			}
		}
		lists[i] = list
	}

	// Column prefix: one delimiter for every earlier column that is not
	// suppressed. A record of a suppressed column is not printed at all.
	prefix := func(col int) string {
		var b strings.Builder
		for c := range col {
			if !p.sup[c] {
				b.WriteString(p.delim)
			}
		}
		return b.String()
	}
	w := bufio.NewWriter(stdout)
	defer w.Flush()
	var counts [3]int
	emit := func(col int, text string) {
		counts[col]++
		if p.sup[col] {
			return
		}
		w.WriteString(prefix(col))
		w.WriteString(text)
		w.WriteString("\n")
	}
	a, b := lists[0], lists[1]
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] < b[j]:
			emit(0, a[i])
			i++
		case a[i] > b[j]:
			emit(1, b[j])
			j++
		default:
			emit(2, a[i])
			i++
			j++
		}
	}
	for ; i < len(a); i++ {
		emit(0, a[i])
	}
	for ; j < len(b); j++ {
		emit(1, b[j])
	}
	if p.total {
		// GNU prints all three counts with the delimiter, then the word
		// "total", even when a column is suppressed (measured 23.11.2026:
		// "4\t6\t0\ttotal").
		fmt.Fprintf(w, "%d%s%d%s%d%stotal\n", counts[0], p.delim, counts[1], p.delim, counts[2], p.delim)
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
