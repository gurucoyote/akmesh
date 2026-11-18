// Package split implements split, cutting a file into pieces.

package split

import (
	"acmesh/internal/shared"

	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	lines := 1000
	bytes := int64(0)
	suffixLen := 2
	numeric := false
	numericFrom := 0
	additional := ""
	chunks := 0
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-d" || a == "--numeric-suffixes":
			numeric = true
			continue
		case strings.HasPrefix(a, "--numeric-suffixes="):
			numeric = true
			fmt.Sscanf(strings.TrimPrefix(a, "--numeric-suffixes="), "%d", &numericFrom)
			continue
		case a == "--additional-suffix":
			if i+1 >= len(args) {
				return 2
			}
			i++
			additional = args[i]
			continue
		case strings.HasPrefix(a, "--additional-suffix="):
			additional = strings.TrimPrefix(a, "--additional-suffix=")
			continue
		case a == "-n" || a == "--number":
			if i+1 >= len(args) {
				return 2
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				fmt.Fprintln(stderr, "split: bad number of chunks")
				return 2
			}
			chunks = n
			continue
		case strings.HasPrefix(a, "-n") && len(a) > 2:
			n, err := strconv.Atoi(a[2:])
			if err != nil || n <= 0 {
				fmt.Fprintln(stderr, "split: bad number of chunks")
				return 2
			}
			chunks = n
			continue
		}
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
			n, err := shared.ParseSize(args[i], 0)
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
		stem := prefix
		if numeric {
			stem += fmt.Sprintf("%0*d", suffixLen, numericFrom+index)
		} else {
			stem += suffixName(index, suffixLen)
		}
		name := stem + additional
		index++
		return os.Create(name)
	}
	if chunks > 0 {
		return splitChunks(r, chunks, next, stderr)
	}
	if bytes > 0 {
		return splitBytes(r, bytes, next, stderr)
	}
	return splitLines(r, lines, next, stderr)
}

// splitChunks divides the input into CHUNKS files of about equal size, as in
// GNU split -n: a chunk ends after the byte boundary but always at a line end,
// so no line is cut in half.
func splitChunks(r *os.File, chunks int, next func() (*os.File, error), stderr *os.File) int {
	data, err := io.ReadAll(r)
	if err != nil {
		fmt.Fprintf(stderr, "split: %v\n", err)
		return 1
	}
	size := len(data)
	if size == 0 {
		return 0
	}
	chunkSize := (size + chunks - 1) / chunks
	if chunkSize < 1 {
		chunkSize = 1
	}
	for pos := 0; pos < size; {
		out, err := next()
		if err != nil {
			fmt.Fprintf(stderr, "split: %v\n", err)
			return 1
		}
		end := pos + chunkSize
		if end > size {
			end = size
		}
		if end < size && data[end-1] != '\n' {
			// We stopped inside a line: finish it, but never add a line that
			// belongs to the next chunk.
			if nl := bytes.IndexByte(data[end:], '\n'); nl >= 0 {
				end += nl + 1
			} else {
				end = size
			}
		}
		if _, err := out.Write(data[pos:end]); err != nil {
			out.Close()
			fmt.Fprintf(stderr, "split: %v\n", err)
			return 1
		}
		out.Close()
		pos = end
	}
	return 0
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
