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

func decodeone(stdin io.Reader, stdout io.Writer) error {
	r := base64.NewDecoder(base64.StdEncoding, stdin)
	if _, err := io.Copy(stdout, r); err != nil {
		return fmt.Errorf("decoding: %w", err)
	}
	return nil
}

// wrap writes text in lines of the given width; 0 means one single line.
func wrap(stdout io.Writer, data []byte, width int) error {
	if width <= 0 {
		if _, err := stdout.Write(data); err != nil {
			return err
		}
		_, err := fmt.Fprintln(stdout)
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

// decodeIgnoring drops every byte outside the base64 alphabet and the padding,
// which is what "base64 -i" does.
func decodeIgnoring(stdin io.Reader, stdout io.Writer) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return err
	}
	kept := make([]byte, 0, len(data))
	for _, b := range data {
		if b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9' || b == '+' || b == '/' || b == '=' {
			kept = append(kept, b)
		}
	}
	decoded, err := base64.StdEncoding.DecodeString(string(kept))
	if err != nil {
		return err
	}
	_, err = stdout.Write(decoded)
	return err
}

func encodeUpstream(stdin io.Reader, stdout io.Writer) error {
	// WriteCloser is important here, from NewEncoder documentation:
	// when finished writing, the caller must Close the returned encoder
	// to flush any partially written blocks.
	wc := base64.NewEncoder(base64.StdEncoding, stdout)
	defer wc.Close()
	if _, err := io.Copy(wc, stdin); err != nil {
		return fmt.Errorf("error encoding: %w", err)
	}
	if err := wc.Close(); err != nil { // flush any remaining data
		return fmt.Errorf("closing encoder: %w", err)
	}
	if _, err := fmt.Fprintln(stdout); err != nil {
		return fmt.Errorf("error encoder writing trailing newline %w", err)
	}
	return nil
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
					return p, nil, fmt.Errorf("unknown option %q", "-"+string(letters[j]))
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
		fmt.Fprintf(stderr, "base64: %v\n", err)
		return 1
	}
	if len(rest) > 1 {
		fmt.Fprintf(stderr, "base64: %v\n", errBadUsage)
		return 1
	}
	if len(rest) == 1 && rest[0] != "-" {
		f, err := os.Open(rest[0])
		if err != nil {
			fmt.Fprintf(stderr, "base64: %v\n", err)
			return 1
		}
		defer f.Close()
		stdin = f
	}
	if p.decode {
		if p.ignore {
			if err := decodeIgnoring(stdin, stdout); err != nil {
				fmt.Fprintf(stderr, "base64: %v\n", err)
				return 1
			}
			return 0
		}
		if err := decodeone(stdin, stdout); err != nil {
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
