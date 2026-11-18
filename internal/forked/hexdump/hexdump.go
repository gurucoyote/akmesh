// Copyright 2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// hexdump prints file content in hexadecimal.
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/hexdump/hexdump.go.
// The rest of this file is the upstream source and keeps its copyright.
//
// Local changes, all of them for the way agents call hexdump:
//   - package hexdump and Run(args, stdin, stdout, stderr) instead of main();
//   - new: -C, the canonical form every agent writes. It is laid out exactly
//     like GNU's: offset, 16 byte slots in two groups of eight, then the
//     printable characters between bars;
//   - -n LENGTH limits the output, as GNU does;
//   - a file that cannot be opened is named and exits 1.
package hexdump

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func hexdump(filenames []string, reader io.Reader, writer io.Writer) error {
	var readers []io.Reader

	if len(filenames) == 0 {
		readers = []io.Reader{reader}
	} else {
		readers = make([]io.Reader, 0, len(filenames))

		for _, filename := range filenames {
			f, err := os.Open(filename)
			if err != nil {
				return err
			}
			defer f.Close()
			readers = append(readers, f)
		}
	}

	r := io.MultiReader(readers...)
	w := hex.Dumper(writer)
	defer w.Close()

	if _, err := io.Copy(w, r); err != nil {
		return err
	}

	return nil
}

// canonical writes the form of "hexdump -C": the offset, the bytes in two
// groups of eight, and the printable characters between bars.
func canonical(r io.Reader, w io.Writer) error {
	buf := make([]byte, 16)
	var offset int64
	for {
		n, err := io.ReadFull(r, buf)
		if err == io.ErrUnexpectedEOF || err == io.EOF {
			if n == 0 {
				break
			}
		} else if err != nil {
			return err
		}
		line := buf[:n]
		field := &strings.Builder{}
		for i := 0; i < 16; i++ {
			if i < n {
				fmt.Fprintf(field, "%02x ", line[i])
			} else {
				field.WriteString("   ")
			}
			if i == 7 {
				field.WriteByte(' ')
			}
		}
		text := &strings.Builder{}
		for _, b := range line {
			if b >= 32 && b < 127 {
				text.WriteByte(b)
			} else {
				text.WriteByte('.')
			}
		}
		fmt.Fprintf(w, "%08x  %s |%s|\n", offset, field.String(), text.String())
		offset += int64(n)
	}
	fmt.Fprintf(w, "%08x\n", offset)
	return nil
}

// Run implements hexdump for the acmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	canonicalOut := false
	var limit int64
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case a == "-C" || a == "--canonical":
			canonicalOut = true
		case a == "-n" || strings.HasPrefix(a, "-n"):
			value := strings.TrimPrefix(a, "-n")
			if value == "" {
				if i+1 >= len(args) {
					fmt.Fprintln(stderr, "hexdump: -n needs a value")
					return 1
				}
				i++
				value = args[i]
			}
			n, err := strconv.ParseInt(value, 0, 64)
			if err != nil {
				fmt.Fprintf(stderr, "hexdump: invalid length %q\n", value)
				return 1
			}
			limit = n
		case strings.HasPrefix(a, "-") && len(a) > 1:
			fmt.Fprintf(stderr, "hexdump: unknown option %q\n", a)
			return 1
		default:
			files = append(files, a)
		}
	}

	if !canonicalOut {
		if err := hexdump(files, stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "hexdump: %v\n", err)
			return 1
		}
		return 0
	}

	var r io.Reader = stdin
	if len(files) > 0 {
		var readers []io.Reader
		var closers []*os.File
		for _, name := range files {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(stderr, "hexdump: %v\n", err)
				return 1
			}
			closers = append(closers, f)
			readers = append(readers, f)
		}
		defer func() {
			for _, f := range closers {
				f.Close()
			}
		}()
		r = io.MultiReader(readers...)
	}
	if limit > 0 {
		r = io.LimitReader(r, limit)
	}
	if err := canonical(r, stdout); err != nil {
		fmt.Fprintf(stderr, "hexdump: %v\n", err)
		return 1
	}
	return 0
}
