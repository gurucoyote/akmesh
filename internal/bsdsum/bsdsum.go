// Package bsdsum is the historic sum command: the BSD rotating checksum by
// default and the System V 16 bit sum with -s, as BusyBox and coreutils print
// it. It is not a hash - md5sum and sha256sum live in internal/sum - but old
// scripts still call it, and an agent that cannot sum reaches for python
// (acmesh-vgr). The package is named bsdsum because internal/sum already
// carries the checksum family and sum is the command name, not a family.
package bsdsum

import (
	"fmt"
	"io"
	"os"
)

// Run implements sum [-r] [-s] [FILE...]; no file means standard input.
func Run(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	sysv := false
	var files []string
	for _, a := range args {
		switch a {
		case "-s", "--sysv":
			sysv = true
		case "-r", "--bsd":
			sysv = false
		default:
			if len(a) > 1 && a[0] == '-' && a != "-" {
				fmt.Fprintf(stderr, "sum: unrecognized option '%s'\n", a)
				fmt.Fprintln(stderr, "Try 'sum --help' for more information.")
				return 1
			}
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	code := 0
	for _, name := range files {
		var r io.Reader = stdin
		if name != "-" {
			f, err := os.Open(name)
			if err != nil {
				fmt.Fprintf(stderr, "sum: %v\n", err)
				code = 1
				continue
			}
			defer f.Close()
			r = f
		}
		sum, blocks, err := checksum(r, sysv)
		if err != nil {
			fmt.Fprintf(stderr, "sum: %s: %v\n", name, err)
			code = 1
			continue
		}
		if sysv {
			fmt.Fprintf(stdout, "%d %d %s\n", sum, blocks, name)
			continue
		}
		// BusyBox pads the checksum to five digits and right aligns the block
		// count; scripts parse by column, so the shape is kept.
		fmt.Fprintf(stdout, "%05d %5d %s\n", sum, blocks, name)
	}
	return code
}

// checksum reads everything and returns the checksum plus the size in 1 KiB
// blocks, rounded up as the historic tools do.
func checksum(r io.Reader, sysv bool) (uint32, int64, error) {
	buf := make([]byte, 32*1024)
	var bsd, sv uint32
	var size int64
	for {
		n, err := r.Read(buf)
		for _, b := range buf[:n] {
			bsd = (bsd>>1 | bsd<<15) + uint32(b) // rotate right, then add
			bsd &= 0xffff
			sv += uint32(b)
		}
		size += int64(n)
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, 0, err
		}
	}
	blocks := (size + 1023) / 1024
	if sysv {
		// Fold the 32 bit sum into 16 bits until nothing is left above.
		for sv > 0xffff {
			sv = (sv & 0xffff) + (sv >> 16)
		}
		return sv, blocks, nil
	}
	return bsd, blocks, nil
}
