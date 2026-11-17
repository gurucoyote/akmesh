package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// runStat implements a compact stat: stat [-c FORMAT] FILE...
// Supported format directives: %n name, %s size, %a octal mode, %i inode,
// %y modification time, %F file type, %% percent.
func runStat(args []string, stdin *os.File, stdout, stderr *os.File) int {
	format := ""
	var files []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-c", "--format":
			if i+1 >= len(args) {
				return 2
			}
			i++
			format = args[i]
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				fmt.Fprintf(stderr, "stat: unknown option %q\n", args[i])
				return 2
			}
			files = append(files, args[i])
		}
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, "stat: missing operand")
		return 2
	}
	code := 0
	for _, name := range files {
		info, err := os.Stat(name)
		if err != nil {
			fmt.Fprintf(stderr, "stat: %v\n", err)
			code = 1
			continue
		}
		if format == "" {
			fmt.Fprintf(stdout, "%s %d %04o %s\n", name, info.Size(), info.Mode().Perm(), info.ModTime().Format("2006-01-02 15:04:05"))
			continue
		}
		out := format
		out = strings.ReplaceAll(out, "%n", name)
		out = strings.ReplaceAll(out, "%s", strconv.FormatInt(info.Size(), 10))
		out = strings.ReplaceAll(out, "%a", strconv.FormatUint(uint64(info.Mode().Perm()), 8))
		out = strings.ReplaceAll(out, "%i", inodeOf(info))
		out = strings.ReplaceAll(out, "%y", info.ModTime().Format("2006-01-02 15:04:05 -0700"))
		out = strings.ReplaceAll(out, "%F", fileTypeOf(info))
		out = strings.ReplaceAll(out, "%%", "%")
		fmt.Fprintln(stdout, out)
	}
	return code
}

func fileTypeOf(info os.FileInfo) string {
	switch {
	case info.IsDir():
		return "directory"
	case info.Mode()&os.ModeSymlink != 0:
		return "symbolic link"
	case info.Mode().IsRegular():
		return "regular file"
	default:
		return "special file"
	}
}

// runTruncate implements truncate -s SIZE FILE...
// SIZE accepts K, M, G suffixes and a leading + or - for relative sizes.
func runTruncate(args []string, stdin *os.File, stdout, stderr *os.File) int {
	size := ""
	var files []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-s", "--size":
			if i+1 >= len(args) {
				return 2
			}
			i++
			size = args[i]
		case "-c", "--no-create":
			fmt.Fprintln(stderr, "truncate: -c is not supported")
			return 2
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				fmt.Fprintf(stderr, "truncate: unknown option %q\n", args[i])
				return 2
			}
			files = append(files, args[i])
		}
	}
	if size == "" || len(files) == 0 {
		fmt.Fprintln(stderr, "usage: truncate -s SIZE FILE...")
		return 2
	}
	for _, name := range files {
		if err := truncateTo(name, size); err != nil {
			fmt.Fprintf(stderr, "truncate: %v\n", err)
			return 1
		}
	}
	return 0
}

func truncateTo(name, size string) error {
	info, err := os.Stat(name)
	if err != nil {
		return err
	}
	cur := info.Size()
	target, err := parseSize(size, cur)
	if err != nil {
		return err
	}
	if target < 0 {
		return fmt.Errorf("%s: negative size", name)
	}
	if target > cur {
		f, err := os.OpenFile(name, os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer f.Close()
		if err := f.Truncate(target); err != nil {
			return err
		}
		return f.Sync()
	}
	return os.Truncate(name, target)
}

func parseSize(s string, cur int64) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty size")
	}
	rel := byte(0)
	switch s[0] {
	case '+', '-':
		rel = s[0]
		s = s[1:]
	}
	mult := int64(1)
	if len(s) > 0 {
		switch s[len(s)-1] {
		case 'K', 'k':
			mult = 1024
			s = s[:len(s)-1]
		case 'M', 'm':
			mult = 1024 * 1024
			s = s[:len(s)-1]
		case 'G', 'g':
			mult = 1024 * 1024 * 1024
			s = s[:len(s)-1]
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad size %q", s)
	}
	n *= mult
	switch rel {
	case '+':
		return cur + n, nil
	case '-':
		return cur - n, nil
	}
	return n, nil
}

// runSplit implements split [-l LINES] [-b SIZE] [-a LEN] [FILE] [PREFIX]
func runSplit(args []string, stdin *os.File, stdout, stderr *os.File) int {
	lines := 1000
	bytes := int64(0)
	suffixLen := 2
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-l", "--lines":
			if i+1 >= len(args) {
				return 2
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				fmt.Fprintln(stderr, "split: bad line count")
				return 2
			}
			lines = n
		case "-b", "--bytes":
			if i+1 >= len(args) {
				return 2
			}
			i++
			n, err := parseSize(args[i], 0)
			if err != nil || n <= 0 {
				fmt.Fprintln(stderr, "split: bad byte count")
				return 2
			}
			bytes = n
		case "-a", "--suffix-length":
			if i+1 >= len(args) {
				return 2
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 || n > 8 {
				fmt.Fprintln(stderr, "split: bad suffix length")
				return 2
			}
			suffixLen = n
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				fmt.Fprintf(stderr, "split: unknown option %q\n", args[i])
				return 2
			}
			positional = append(positional, args[i])
		}
	}
	input := "-"
	prefix := "x"
	if len(positional) > 0 {
		input = positional[0]
	}
	if len(positional) > 1 {
		prefix = positional[1]
	}
	var r *os.File = stdin
	if input != "-" {
		f, err := os.Open(input)
		if err != nil {
			fmt.Fprintf(stderr, "split: %v\n", err)
			return 1
		}
		defer f.Close()
		r = f
	}

	index := 0
	capacity := 1
	for i := 0; i < suffixLen; i++ {
		capacity *= len(suffixAlphabet)
	}
	next := func() (*os.File, error) {
		if index >= capacity {
			return nil, fmt.Errorf("output file suffixes exhausted")
		}
		name := prefix + suffixName(index, suffixLen)
		index++
		return os.Create(name)
	}
	if bytes > 0 {
		return splitBytes(r, bytes, next, stderr)
	}
	return splitLines(r, lines, next, stderr)
}

func splitLines(r *os.File, lines int, next func() (*os.File, error), stderr *os.File) int {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	var out *os.File
	count := 0
	for sc.Scan() {
		if out == nil || count == lines {
			if out != nil {
				out.Close()
			}
			var err error
			out, err = next()
			if err != nil {
				fmt.Fprintf(stderr, "split: %v\n", err)
				return 1
			}
			count = 0
		}
		fmt.Fprintln(out, sc.Text())
		count++
	}
	if out != nil {
		out.Close()
	}
	return 0
}

func splitBytes(r *os.File, size int64, next func() (*os.File, error), stderr *os.File) int {
	buf := make([]byte, 32*1024)
	var out *os.File
	var written int64
	for {
		n, err := r.Read(buf)
		for n > 0 {
			if out == nil || written == size {
				if out != nil {
					out.Close()
				}
				var cerr error
				out, cerr = next()
				if cerr != nil {
					fmt.Fprintf(stderr, "split: %v\n", cerr)
					return 1
				}
				written = 0
			}
			chunk := int64(n)
			if chunk > size-written {
				chunk = size - written
			}
			if _, werr := out.Write(buf[:chunk]); werr != nil {
				fmt.Fprintf(stderr, "split: %v\n", werr)
				return 1
			}
			written += chunk
			buf = buf[chunk:]
			n -= int(chunk)
		}
		if err != nil {
			break
		}
	}
	if out != nil {
		out.Close()
	}
	return 0
}

const suffixAlphabet = "abcdefghijklmnopqrstuvwxyz"

func suffixName(index, length int) string {
	out := make([]byte, length)
	for i := length - 1; i >= 0; i-- {
		out[i] = suffixAlphabet[index%len(suffixAlphabet)]
		index /= len(suffixAlphabet)
	}
	return string(out)
}

// runSync makes sure written data reaches the device for the named files.
// A whole-system sync does not exist on Windows; there we say so instead of
// pretending.
func runSync(args []string, stdin *os.File, stdout, stderr *os.File) int {
	if len(args) == 0 {
		if wholeSystemSync() != nil {
			fmt.Fprintln(stderr, "sync: not supported on this platform: pass FILE operands to sync specific files")
			return 1
		}
		return 0
	}
	for _, name := range args {
		f, err := os.OpenFile(name, os.O_RDWR, 0o644)
		if err != nil {
			fmt.Fprintf(stderr, "sync: %v\n", err)
			return 1
		}
		err = f.Sync()
		f.Close()
		if err != nil {
			fmt.Fprintf(stderr, "sync: %v\n", err)
			return 1
		}
	}
	return 0
}
