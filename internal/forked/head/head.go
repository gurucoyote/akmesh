// Copyright 2024 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/head/head.go.
// The rest of this file is the upstream source and keeps its copyright.
//
// Local changes, all of them for agent compatibility:
//   - package main and main() became package head and Run(args, stdin, stdout,
//     stderr), so the binary's dispatcher can call it;
//   - the flag package is gone: agents write -5 and -q, which flag reads as an
//     unknown single-dash name, and GNU accepts both;
//   - -q suppresses the ==> name <== headers, -v forces them on one file;
//   - a read error exits 1 (upstream printed it and returned success);
//   - an unknown option prints the usage and exits 2.

package head

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

var errCombine = fmt.Errorf("can't combine line and byte counts")

func run(stdin io.Reader, stdout, stderr io.Writer, quiet, verbose bool, bytes, count int, files ...string) error {
	if bytes > 0 && count > 0 {
		return errCombine
	}

	var printBytes bool
	var buffer []byte
	if bytes > 0 {
		printBytes = true
		buffer = make([]byte, 4096)
	}

	if count == 0 {
		count = 10
	}

	var newLineHeader bool
	var errs error

	// Headers appear for more than one file, always with -v, never with -q.
	headers := len(files) > 1 && !quiet
	if verbose {
		headers = len(files) > 0
	}

	handle := func(r io.Reader, name string) error {
		if headers {
			if newLineHeader {
				fmt.Fprintf(stdout, "\n==> %s <==\n", name)
			} else {
				fmt.Fprintf(stdout, "==> %s <==\n", name)
				newLineHeader = true
			}
		}
		if printBytes {
			c := bytes
			for {
				n, err := io.ReadFull(r, buffer)
				if err == io.EOF {
					break
				}
				if err != nil && err != io.ErrUnexpectedEOF {
					return err
				}

				stdout.Write(buffer[:min(c, n)])
				c -= n
				if c <= 0 {
					break
				}

				// handle the case when user request more bytes than
				// source have
				if err == io.ErrUnexpectedEOF {
					break
				}
			}
		} else {
			var c int
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				fmt.Fprintln(stdout, scanner.Text())
				c++
				if c == count {
					break
				}
			}
		}
		return nil
	}

	// handle stdin
	if len(files) == 0 {
		return handle(stdin, "")
	}

	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			errs = errors.Join(errs, fmt.Errorf("head: %w", err))
			continue
		}
		err = handle(f, f.Name())
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}

	if errs != nil {
		// Upstream already prefixes with "head:", so no second one here.
		fmt.Fprintf(stderr, "%v\n", errs)
	}
	return errs
}

// parseCount accepts the GNU spellings: 5, -5, 10k, +3.
func parseCount(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty count")
	}
	mult := 1
	switch s[len(s)-1] {
	case 'k', 'K':
		mult = 1024
		s = s[:len(s)-1]
	case 'm', 'M':
		mult = 1024 * 1024
		s = s[:len(s)-1]
	}
	s = strings.TrimPrefix(s, "+")
	s = strings.TrimPrefix(s, "-")
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid number of lines: %q", s)
	}
	return n * mult, nil
}

func usage(stderr io.Writer) {
	fmt.Fprintln(stderr, "usage: head [-n NUM] [-c NUM] [-q] [-v] [FILE...]")
}

// Run implements head for the acmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	bytes, count := 0, 0
	quiet, verbose := false, false
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case a == "-q" || a == "--quiet" || a == "--silent":
			quiet = true
		case a == "-v" || a == "--verbose":
			verbose = true
		case a == "-n" || a == "--lines":
			if i+1 >= len(args) {
				usage(stderr)
				return 2
			}
			i++
			n, err := parseCount(args[i])
			if err != nil {
				fmt.Fprintf(stderr, "head: %v\n", err)
				return 2
			}
			count = n
		case strings.HasPrefix(a, "--lines="):
			n, err := parseCount(strings.TrimPrefix(a, "--lines="))
			if err != nil {
				fmt.Fprintf(stderr, "head: %v\n", err)
				return 2
			}
			count = n
		case a == "-c" || a == "--bytes":
			if i+1 >= len(args) {
				usage(stderr)
				return 2
			}
			i++
			n, err := parseCount(args[i])
			if err != nil {
				fmt.Fprintf(stderr, "head: %v\n", err)
				return 2
			}
			bytes = n
		case strings.HasPrefix(a, "--bytes="):
			n, err := parseCount(strings.TrimPrefix(a, "--bytes="))
			if err != nil {
				fmt.Fprintf(stderr, "head: %v\n", err)
				return 2
			}
			bytes = n
		case strings.HasPrefix(a, "-n") && len(a) > 2:
			n, err := parseCount(a[2:])
			if err != nil {
				fmt.Fprintf(stderr, "head: %v\n", err)
				return 2
			}
			count = n
		case strings.HasPrefix(a, "-c") && len(a) > 2:
			n, err := parseCount(a[2:])
			if err != nil {
				fmt.Fprintf(stderr, "head: %v\n", err)
				return 2
			}
			bytes = n
		case strings.HasPrefix(a, "-") && len(a) > 1:
			// The GNU short form: head -5, and the combined -q5.
			rest := a[1:]
			for len(rest) > 0 && (rest[0] == 'q' || rest[0] == 'v') {
				if rest[0] == 'q' {
					quiet = true
				} else {
					verbose = true
				}
				rest = rest[1:]
			}
			n, err := parseCount(rest)
			if rest == "" || err != nil {
				fmt.Fprintf(stderr, "head: invalid option %q\n", a)
				usage(stderr)
				return 2
			}
			count = n
		default:
			files = append(files, a)
		}
	}
	if bytes > 0 && count > 0 {
		fmt.Fprintf(stderr, "head: %v\n", errCombine)
		return 2
	}
	if err := run(stdin, stdout, stderr, quiet, verbose, bytes, count, files...); err != nil {
		return 1
	}
	return 0
}
