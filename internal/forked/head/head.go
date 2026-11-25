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

func run(stdin io.Reader, stdout, stderr io.Writer, header int, printBytes, zero bool, bytes, count int, files ...string) error {
	var buffer []byte
	if printBytes && bytes > 0 {
		// Only the positive byte count reads in blocks; -c -N needs the end.
		buffer = make([]byte, 4096)
	}
	// A count of zero prints nothing. The caller passes 10 when no -n or -c
	// was given at all, so a real "0" stays a zero.
	if !printBytes && count == 0 {
		return nil
	}

	var newLineHeader bool
	var errs error

	// The last of -q and -v wins, as in GNU. Without either, headers appear
	// for more than one file only.
	headers := len(files) > 1
	switch header {
	case 0:
		headers = false
	case 1:
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
		if printBytes && bytes < 0 {
			// -c -N: everything but the last N bytes.
			all, rerr := io.ReadAll(r)
			if rerr != nil {
				return rerr
			}
			if keep := len(all) + bytes; keep > 0 {
				stdout.Write(all[:keep])
			}
			return nil
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
			// Records go out exactly as they were read: a last line without a
			// newline stays without one and a NUL record keeps its NUL. The old
			// scanner printed every record with Fprintln, so it appended a
			// newline that was not in the file (measured 24.11.2026, acmesh-fqj).
			delim := byte('\n')
			if zero {
				delim = 0
			}
			br := bufio.NewReaderSize(r, 64*1024)
			next := func() (string, error) { return br.ReadString(delim) }
			if count < 0 {
				// -n -N: everything but the last N records, so the ring holds N.
				want := -count
				ring := make([]string, 0, want)
				for {
					rec, err := next()
					if rec != "" {
						if len(ring) == want {
							io.WriteString(stdout, ring[0])
							ring = ring[1:]
						}
						ring = append(ring, rec)
					}
					if err != nil {
						break
					}
				}
				return nil
			}
			for range count {
				rec, err := next()
				if rec != "" {
					io.WriteString(stdout, rec)
				}
				if err != nil {
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
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid number of lines: %q", s)
	}
	// GNU reads "-n -2" as "everything but the last two lines", not as "two".
	if negative {
		return -n * mult, nil
	}
	return n * mult, nil
}

func usage(stderr io.Writer) {
	fmt.Fprintln(stderr, "usage: head [-n NUM] [-c NUM] [-z] [-q] [-v] [FILE...]")
}

// Run implements head for the akmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var bytes, count int
	byBytes, byLines, zero := false, false, false
	header := -1 // -1 unset, 0 from -q, 1 from -v
	var files []string

	// GNU lets the last of -c and -n win, so setting one clears the other.
	setCount := func(n int, isBytes bool) {
		if isBytes {
			bytes, byBytes, byLines = n, true, false
			return
		}
		count, byLines, byBytes = n, true, false
	}
	badCount := func(err error) int {
		fmt.Fprintf(stderr, "head: %v\n", err)
		return 2
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case a == "-q" || a == "--quiet" || a == "--silent":
			header = 0
		case a == "-v" || a == "--verbose":
			header = 1
		case a == "-z" || a == "--zero-terminated":
			zero = true
		case a == "-n" || a == "--lines":
			if i+1 >= len(args) {
				usage(stderr)
				return 2
			}
			i++
			n, err := parseCount(args[i])
			if err != nil {
				return badCount(err)
			}
			setCount(n, false)
		case strings.HasPrefix(a, "--lines="):
			n, err := parseCount(strings.TrimPrefix(a, "--lines="))
			if err != nil {
				return badCount(err)
			}
			setCount(n, false)
		case a == "-c" || a == "--bytes":
			if i+1 >= len(args) {
				usage(stderr)
				return 2
			}
			i++
			n, err := parseCount(args[i])
			if err != nil {
				return badCount(err)
			}
			setCount(n, true)
		case strings.HasPrefix(a, "--bytes="):
			n, err := parseCount(strings.TrimPrefix(a, "--bytes="))
			if err != nil {
				return badCount(err)
			}
			setCount(n, true)
		case strings.HasPrefix(a, "-n") && len(a) > 2:
			n, err := parseCount(a[2:])
			if err != nil {
				return badCount(err)
			}
			setCount(n, false)
		case strings.HasPrefix(a, "-c") && len(a) > 2:
			n, err := parseCount(a[2:])
			if err != nil {
				return badCount(err)
			}
			setCount(n, true)
		case strings.HasPrefix(a, "-") && len(a) > 1:
			// The GNU short form: head -5, and the combined -q5.
			rest := a[1:]
			for len(rest) > 0 && (rest[0] == 'q' || rest[0] == 'v') {
				if rest[0] == 'q' {
					header = 0
				} else {
					header = 1
				}
				rest = rest[1:]
			}
			n, err := parseCount(rest)
			if rest == "" || err != nil {
				fmt.Fprintf(stderr, "head: invalid option %q\n", a)
				usage(stderr)
				return 2
			}
			setCount(n, false)
		default:
			files = append(files, a)
		}
	}
	if !byBytes && !byLines {
		count = 10
	}
	if err := run(stdin, stdout, stderr, header, byBytes, zero, bytes, count, files...); err != nil {
		return 1
	}
	return 0
}
