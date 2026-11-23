// Copyright 2021 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// base64 - encode and decode base64 from stdin or file to stdout
//
// Synopsis:
//
//	base64 [-d] [FILE]
//
// Description:
//
//	Encode or decode a file to or from base64 encoding.
//	-d   decode data (default is to encode)
//	For stdin, on standard Unix systems, you can use /dev/stdin
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/base64/base64.go.
// The rest of this file is the upstream source and keeps its copyright.
//
// Local changes, all of them for the way agents call base64:
//   - package base64 and Run(args, stdin, stdout, stderr) instead of main();
//   - no unixflag: an own parser, so -w0 and -w 0 both work;
//   - new: -w COLS (wrapped output like GNU, default 76, 0 means one line) and
//     -i (ignore characters outside the alphabet while decoding);
//   - the trailing newline is only written when something was encoded.
package base64

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	akusage "akmesh/internal/usage"
)

type cmd struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	params
	args []string
}

type params struct {
	decode bool
	wrap   int
	ignore bool
}

var errBadUsage = errors.New("usage: base64 [-d] [file]")

// errInvalidInput is GNU's wording for data that is not base64 at all.
var errInvalidInput = errors.New("invalid input")

// unknownOption carries the option as the user wrote it, so the message can name
// the whole token and point at --help.
type unknownOption struct{ opt string }

func (e unknownOption) Error() string { return "unrecognized option '" + e.opt + "'" }

// decodeone decodes the stream the way GNU does (measured 23.11.2026,
// acmesh-449):
//   - a line break or CR is part of the format and is skipped anywhere;
//   - a space or tab is garbage: without -i the complete groups before it are
//     written and the run ends with "invalid input"; with -i it is dropped;
//   - a last group of two or three characters has no padding and decodes
//     without an error, because a hash or token is often copied without "=";
//   - a last group of one character is invalid.
func decodeone(stdin io.Reader, stdout io.Writer, ignore bool) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return err
	}
	out := make([]byte, 0, len(data)/4*3)
	var group []byte
	invalid := false
	flush := func(partial bool) bool {
		if len(group) == 0 {
			return true
		}
		var dst [3]byte
		var n int
		var derr error
		if partial {
			n, derr = base64.RawStdEncoding.Decode(dst[:], group)
		} else {
			n, derr = base64.StdEncoding.Decode(dst[:], group)
		}
		if derr != nil {
			return false
		}
		out = append(out, dst[:n]...)
		group = group[:0]
		return true
	}
	for _, b := range data {
		switch {
		case b == '\n' || b == '\r':
			continue
		case b == ' ' || b == '\t':
			if ignore {
				continue
			}
			invalid = true
		case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b >= '0' && b <= '9', b == '+', b == '/', b == '=':
			group = append(group, b)
			if len(group) == 4 {
				if !flush(false) {
					invalid = true
				}
			}
			continue
		default:
			if ignore {
				continue
			}
			invalid = true
		}
		break
	}
	if !invalid && len(group) > 0 {
		// Padding in a short group means it was cut; GNU calls that invalid.
		for _, b := range group {
			if b == '=' {
				invalid = true
			}
		}
		if len(group) == 1 {
			invalid = true
		} else if !invalid && !flush(true) {
			invalid = true
		}
	}
	if _, werr := stdout.Write(out); werr != nil {
		return werr
	}
	if invalid {
		return errInvalidInput
	}
	return nil
}

// wrap writes text in lines of the given width; 0 means one single line.
func wrap(stdout io.Writer, data []byte, width int) error {
	if width <= 0 {
		// GNU appends nothing here: "-w0" is the form a script puts into a
		// variable, and a trailing newline breaks it (acmesh-449).
		_, err := stdout.Write(data)
		return err
	}
	for len(data) > 0 {
		end := width
		if end > len(data) {
			end = len(data)
		}
		if _, err := stdout.Write(data[:end]); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(stdout); err != nil {
			return err
		}
		data = data[end:]
	}
	return nil
}

func encodeone(stdin io.Reader, stdout io.Writer, width int) error {
	var buf bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &buf)
	if _, err := io.Copy(enc, stdin); err != nil {
		return fmt.Errorf("error encoding: %w", err)
	}
	// Close flushes the last partial block; without it the tail of the
	// input is silently missing from the output.
	if err := enc.Close(); err != nil {
		return fmt.Errorf("closing encoder: %w", err)
	}
	return wrap(stdout, buf.Bytes(), width)
}

// parse reads -d, -w and -i, also glued together as -dw 0.
func parse(args []string) (p params, rest []string, err error) {
	p.wrap = 76
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return p, append(rest, args[i+1:]...), nil
		case a == "--decode":
			p.decode = true
		case a == "--ignore-garbage":
			p.ignore = true
		case a == "--wrap" || strings.HasPrefix(a, "--wrap="):
			value := strings.TrimPrefix(strings.TrimPrefix(a, "--wrap"), "=")
			if value == "" {
				if i+1 >= len(args) {
					return p, nil, errors.New("--wrap needs a number")
				}
				i++
				value = args[i]
			}
			n, err := strconv.Atoi(value)
			if err != nil || n < 0 {
				return p, nil, fmt.Errorf("invalid wrap size: '%s'", value)
			}
			p.wrap = n
		case strings.HasPrefix(a, "--"):
			// A long option this parser does not know: name the whole token.
			// Reading only its first letter used to print `unknown option "--"`,
			// which told nobody what was wrong (acmesh-vr1).
			return p, nil, unknownOption{a}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			letters := a[1:]
			for j := 0; j < len(letters); j++ {
				switch letters[j] {
				case 'd':
					p.decode = true
				case 'i':
					p.ignore = true
				case 'w':
					value := letters[j+1:]
					if value == "" {
						if i+1 >= len(args) {
							return p, nil, errors.New("option -w needs a number")
						}
						i++
						value = args[i]
					}
					n, err := strconv.Atoi(value)
					if err != nil || n < 0 {
						return p, nil, fmt.Errorf("invalid wrap size: '%s'", value)
					}
					p.wrap = n
					j = len(letters)
				default:
					return p, nil, unknownOption{"-" + string(letters[j])}
				}
			}
		default:
			rest = append(rest, a)
		}
	}
	return p, rest, nil
}

// Run implements base64 for the akmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	p, rest, err := parse(args)
	if err != nil {
		var uo unknownOption
		if errors.As(err, &uo) {
			return akusage.BadOption("base64", uo.opt, stderr, 1)
		}
		fmt.Fprintf(stderr, "base64: %v\n", err)
		return 1
	}
	if len(rest) > 1 {
		// GNU names the operand it does not want instead of printing a usage.
		fmt.Fprintf(stderr, "base64: extra operand '%s'\n", rest[1])
		fmt.Fprintln(stderr, "Try 'base64 --help' for more information.")
		return 1
	}
	if len(rest) == 1 && rest[0] != "-" {
		f, err := os.Open(rest[0])
		if err != nil {
			var pe *os.PathError
			if errors.As(err, &pe) {
				// GNU capitalises the reason: "No such file or directory".
				reason := pe.Err.Error()
				if reason != "" {
					reason = strings.ToUpper(reason[:1]) + reason[1:]
				}
				fmt.Fprintf(stderr, "base64: %s: %s\n", rest[0], reason)
			} else {
				fmt.Fprintf(stderr, "base64: %v\n", err)
			}
			return 1
		}
		defer f.Close()
		stdin = f
	}
	if p.decode {
		if err := decodeone(stdin, stdout, p.ignore); err != nil {
			fmt.Fprintf(stderr, "base64: %v\n", err)
			return 1
		}
		return 0
	}
	if err := encodeone(stdin, stdout, p.wrap); err != nil {
		fmt.Fprintf(stderr, "base64: %v\n", err)
		return 1
	}
	return 0
}
