package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
)

// runSum builds one checksum command. The output format is the usual
// "<hex>  <name>", with "-" for standard input.
func runSum(kind string) run {
	return func(args []string, stdin *os.File, stdout, stderr *os.File) int {
		var files []string
		check := false
		for _, a := range args {
			switch a {
			case "-c", "--check":
				check = true
			case "-b", "--binary", "-t", "--text":
				// text and binary mode are the same here
			default:
				if len(a) > 0 && a[0] == '-' && a != "-" {
					fmt.Fprintf(stderr, "%ssum: unknown option %q\n", kind, a)
					return 2
				}
				files = append(files, a)
			}
		}
		if check {
			fmt.Fprintf(stderr, "%ssum: -c is not supported yet\n", kind)
			return 2
		}
		if len(files) == 0 {
			files = []string{"-"}
		}
		code := 0
		for _, name := range files {
			sum, err := fileSum(kind, name, stdin)
			if err != nil {
				fmt.Fprintf(stderr, "%ssum: %v\n", kind, err)
				code = 1
				continue
			}
			fmt.Fprintf(stdout, "%s  %s\n", sum, name)
		}
		return code
	}
}

func fileSum(kind, name string, stdin *os.File) (string, error) {
	var r io.Reader = stdin
	if name != "-" {
		f, err := os.Open(name)
		if err != nil {
			return "", err
		}
		defer f.Close()
		r = f
	}
	h, err := newHash(kind)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func newHash(kind string) (hash.Hash, error) {
	switch kind {
	case "md5":
		return md5.New(), nil
	case "sha1":
		return sha1.New(), nil
	case "sha256":
		return sha256.New(), nil
	case "sha512":
		return sha512.New(), nil
	}
	return nil, fmt.Errorf("unknown hash %q", kind)
}
