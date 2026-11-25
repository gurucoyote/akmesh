// Package shufcmd implements shuf(1): write a random permutation of the input
// lines to standard output.
//
// Command line follows GNU shuf: -e, -i LO-HI, -n COUNT, -o FILE, -r, -z,
// --random-source=FILE, --help, --version.
//
// shuf is the uniform permutation tool. sort -R is not the same thing: it
// orders by a hash of the sort key, so identical keys stay grouped.
package shufcmd

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"

	"akmesh/internal/appname"
)

type options struct {
	echo       bool
	inputRange string
	count      string
	output     string
	repeat     bool
	zeroTerm   bool
	randSource string
}

// Run is the entry point the akmesh table calls.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	o, files, errMsg := parseArgs(args)
	if errMsg != "" {
		switch errMsg {
		case "help":
			fmt.Fprint(stdout, usageText)
			return 0
		case "version":
			fmt.Fprintln(stdout, "shuf ("+appname.Name+") 1.0")
			return 0
		}
		fmt.Fprintf(stderr, "shuf: %s\n", errMsg)
		fmt.Fprintln(stderr, "shuf: try 'shuf --help' for the supported options")
		return 1
	}
	if o.echo && o.inputRange != "" {
		fmt.Fprintln(stderr, "shuf: cannot combine -e and -i options")
		return 1
	}

	src, err := newSource(o.randSource)
	if err != nil {
		fmt.Fprintf(stderr, "shuf: %s: %v\n", o.randSource, err)
		return 1
	}

	var lines []string
	switch {
	case o.echo:
		lines = files
		files = nil
	case o.inputRange != "":
		lo, hi, ok := parseRange(o.inputRange)
		if !ok {
			fmt.Fprintf(stderr, "shuf: invalid input range: %s\n", o.inputRange)
			return 1
		}
		for i := lo; i <= hi; i++ {
			lines = append(lines, strconv.Itoa(i))
		}
		files = nil
	default:
		if len(files) == 0 {
			files = []string{"-"}
		}
		lines, err = readLines(files, stdin, o.zeroTerm)
		if err != nil {
			fmt.Fprintf(stderr, "shuf: %v\n", err)
			return 1
		}
	}

	count := len(lines)
	if o.count != "" {
		n, err := strconv.Atoi(o.count)
		if err != nil || n < 0 {
			fmt.Fprintf(stderr, "shuf: invalid line count: %s\n", o.count)
			return 1
		}
		count = n
	}
	if o.repeat && count > 0 && len(lines) == 0 {
		fmt.Fprintln(stderr, "shuf: no lines to repeat")
		return 1
	}

	out := make([]string, 0, count)
	if o.repeat {
		for i := 0; i < count; i++ {
			out = append(out, lines[src.intn(len(lines))])
		}
	} else {
		perm := append([]string(nil), lines...)
		shuffle(perm, src)
		if count < len(perm) {
			perm = perm[:count]
		}
		out = perm
	}

	sep := byte('\n')
	if o.zeroTerm {
		sep = 0
	}
	var buf []byte
	for _, l := range out {
		buf = append(buf, l...)
		buf = append(buf, sep)
	}
	if o.output != "" {
		if err := os.WriteFile(o.output, buf, 0o644); err != nil {
			fmt.Fprintf(stderr, "shuf: %s: %v\n", o.output, err)
			return 1
		}
		return 0
	}
	if _, err := stdout.Write(buf); err != nil {
		fmt.Fprintf(stderr, "shuf: write failed: %v\n", err)
		return 1
	}
	return 0
}

// source yields uniformly distributed indices. It reads from --random-source
// when given, so a test or a script can make a run reproducible.
type source struct {
	file *os.File
}

func newSource(path string) (*source, error) {
	if path == "" {
		return &source{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &source{file: f}, nil
}

// intn returns a number in [0, n).
func (s *source) intn(n int) int {
	if n <= 1 {
		return 0
	}
	if s.file == nil {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
		if err != nil {
			// crypto/rand failing is not something to paper over with a
			// predictable fallback; the caller asked for a shuffle.
			return 0
		}
		return int(v.Int64())
	}
	b := make([]byte, 8)
	if _, err := s.file.Read(b); err != nil {
		return 0
	}
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return int(v % uint64(n))
}

// shuffle is Fisher-Yates, so every permutation is equally likely.
func shuffle(s []string, src *source) {
	for i := len(s) - 1; i > 0; i-- {
		j := src.intn(i + 1)
		s[i], s[j] = s[j], s[i]
	}
}

func parseRange(s string) (int, int, bool) {
	loStr, hiStr, ok := strings.Cut(s, "-")
	if !ok {
		return 0, 0, false
	}
	lo, err1 := strconv.Atoi(strings.TrimSpace(loStr))
	hi, err2 := strconv.Atoi(strings.TrimSpace(hiStr))
	if err1 != nil || err2 != nil || hi < lo {
		return 0, 0, false
	}
	return lo, hi, true
}

func readLines(names []string, stdin *os.File, zeroTerm bool) ([]string, error) {
	sep := byte('\n')
	if zeroTerm {
		sep = 0
	}
	var lines []string
	for _, name := range names {
		var data []byte
		var err error
		if name == "-" {
			data, err = io.ReadAll(stdin)
		} else {
			data, err = os.ReadFile(name)
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %v", name, err)
		}
		for len(data) > 0 {
			i := bytes.IndexByte(data, sep)
			if i < 0 {
				lines = append(lines, string(data))
				break
			}
			lines = append(lines, string(data[:i]))
			data = data[i+1:]
		}
	}
	return lines, nil
}

func parseArgs(args []string) (*options, []string, string) {
	o := &options{}
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			files = append(files, args[i+1:]...)
			break
		}
		if strings.HasPrefix(a, "--") {
			name, val, hasVal := strings.Cut(a[2:], "=")
			need := func() (string, string) {
				if hasVal {
					return val, ""
				}
				if i+1 >= len(args) {
					return "", fmt.Sprintf("option '--%s' requires an argument", name)
				}
				i++
				return args[i], ""
			}
			var e string
			switch name {
			case "help":
				return nil, nil, "help"
			case "version":
				return nil, nil, "version"
			case "echo":
				o.echo = true
			case "repeat":
				o.repeat = true
			case "zero-terminated":
				o.zeroTerm = true
			case "input-range":
				if o.inputRange, e = need(); e != "" {
					return nil, nil, e
				}
			case "head-count":
				if o.count, e = need(); e != "" {
					return nil, nil, e
				}
			case "output":
				if o.output, e = need(); e != "" {
					return nil, nil, e
				}
			case "random-source":
				if o.randSource, e = need(); e != "" {
					return nil, nil, e
				}
			default:
				return nil, nil, fmt.Sprintf("unrecognized option '--%s'", name)
			}
			continue
		}
		if len(a) > 1 && a[0] == '-' {
			for j := 1; j < len(a); j++ {
				c := a[j]
				rest := a[j+1:]
				take := func(name string) (string, string) {
					if rest != "" {
						return rest, ""
					}
					if i+1 >= len(args) {
						return "", fmt.Sprintf("option requires an argument -- %s", name)
					}
					i++
					return args[i], ""
				}
				switch c {
				case 'e':
					o.echo = true
				case 'r':
					o.repeat = true
				case 'z':
					o.zeroTerm = true
				case 'i', 'n', 'o':
					v, err := take(string(c))
					if err != "" {
						return nil, nil, err
					}
					switch c {
					case 'i':
						o.inputRange = v
					case 'n':
						o.count = v
					case 'o':
						o.output = v
					}
					j = len(a)
				default:
					return nil, nil, fmt.Sprintf("invalid option -- %q", string(c))
				}
			}
			continue
		}
		files = append(files, a)
	}
	return o, files, ""
}

const usageText = `Usage: shuf [OPTION]... [FILE]
  or:  shuf -e [OPTION]... [ARG]...
  or:  shuf -i LO-HI [OPTION]...
Write a random permutation of the input lines to standard output.

  -e, --echo                 treat each ARG as an input line
  -i, --input-range=LO-HI    treat each number LO through HI as an input line
  -n, --head-count=COUNT     output at most COUNT lines
  -o, --output=FILE          write result to FILE instead of standard output
  -r, --repeat               output lines can be repeated
      --random-source=FILE   get random bytes from FILE
  -z, --zero-terminated      line delimiter is NUL, not newline
      --help                 display this help and exit
      --version              output version information and exit
`
