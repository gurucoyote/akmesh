// Package odump is od: the octal/hex/character dump an agent types when it wants
// to see bytes ("od -An -tx1"). hexdump -C shows the same bytes in another
// layout, so both are kept (acmesh-2yc).
package odump

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

// format is one -t specification.
type format struct {
	kind  byte // a, c, d, o, u, x, f
	size  int  // 1, 2, 4, 8
	width int  // printed width of one field
}

// Run implements od [-A RADIX] [-t TYPE]... [-N BYTES] [-j BYTES] [-v] [FILE]
func Run(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	radix := 'o' // the historic default: octal offsets
	formats := []format{{kind: 'o', size: 2, width: 6}}
	squash := true
	var limit, skip int64 = -1, 0
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-v" || a == "--output-duplicates":
			squash = false
		case strings.HasPrefix(a, "-A"):
			v := strings.TrimPrefix(a, "-A")
			if v == "" {
				i++
				if i >= len(args) {
					return bad(stderr, "-A needs a radix")
				}
				v = args[i]
			}
			switch v {
			case "d", "o", "x", "n":
				radix = rune(v[0])
			default:
				return bad(stderr, "-A "+v+": unknown radix")
			}
		case strings.HasPrefix(a, "-t"):
			v := strings.TrimPrefix(a, "-t")
			if v == "" {
				i++
				if i >= len(args) {
					return bad(stderr, "-t needs a type")
				}
				v = args[i]
			}
			f, err := parseType(v)
			if err != nil {
				return bad(stderr, err.Error())
			}
			if len(formats) == 1 && formats[0].kind == 'o' && formats[0].size == 2 {
				formats = nil // the default falls away as soon as one -t is given
			}
			formats = append(formats, f)
		case strings.HasPrefix(a, "-N"):
			v, ok := valueOrNext(a, "-N", args, &i)
			if !ok {
				return bad(stderr, "-N needs a count")
			}
			n, err := parseCount(v)
			if err != nil {
				return bad(stderr, err.Error())
			}
			limit = n
		case strings.HasPrefix(a, "-j"):
			v, ok := valueOrNext(a, "-j", args, &i)
			if !ok {
				return bad(stderr, "-j needs a count")
			}
			n, err := parseCount(v)
			if err != nil {
				return bad(stderr, err.Error())
			}
			skip = n
		case a == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case len(a) > 1 && a[0] == '-':
			return bad(stderr, "unrecognized option '"+a+"'")
		default:
			files = append(files, a)
		}
	}
	var r io.Reader = stdin
	var closers []io.Closer
	for _, name := range files {
		if name == "-" {
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(stderr, "od: %v\n", err)
			return 1
		}
		closers = append(closers, f)
		r = f
		break
	}
	defer func() {
		for _, c := range closers {
			c.Close()
		}
	}()
	if skip > 0 {
		if _, err := io.CopyN(io.Discard, r, skip); err != nil && err != io.EOF {
			fmt.Fprintf(stderr, "od: %v\n", err)
			return 1
		}
	}
	data, err := readLimit(r, limit)
	if err != nil {
		fmt.Fprintf(stderr, "od: %v\n", err)
		return 1
	}
	dump(stdout, data, radix, formats, squash, skip)
	return 0
}

// bad prints an od error and returns the exit code od uses for a usage error.
func bad(stderr io.Writer, msg string) int {
	fmt.Fprintf(stderr, "od: %s\n", msg)
	return 1
}

func valueOrNext(a, flag string, args []string, i *int) (string, bool) {
	if v := strings.TrimPrefix(a, flag); v != "" {
		return v, true
	}
	*i++
	if *i >= len(args) {
		return "", false
	}
	return args[*i], true
}

// parseCount reads a decimal or 0x hexadecimal count.
func parseCount(s string) (int64, error) {
	base := 10
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		base = 16
		s = s[2:]
	}
	n, err := strconv.ParseInt(s, base, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid number %q", s)
	}
	return n, nil
}

// parseType reads one -t value such as x1, c, d4 or a.
func parseType(s string) (format, error) {
	if s == "" {
		return format{}, fmt.Errorf("-t without a type")
	}
	switch s[0] {
	case 'a':
		return format{kind: 'a', size: 1}, nil
	case 'c':
		return format{kind: 'c', size: 1}, nil
	case 'd', 'o', 'u', 'x', 'f':
		size := 4
		if len(s) > 1 {
			n, err := strconv.Atoi(s[1:])
			if err != nil {
				return format{}, fmt.Errorf("-t %s: unknown size", s)
			}
			size = n
		}
		if s[0] == 'f' && size != 4 && size != 8 {
			return format{}, fmt.Errorf("-t %s: floats are 4 or 8 bytes", s)
		}
		if s[0] != 'f' && size != 1 && size != 2 && size != 4 && size != 8 {
			return format{}, fmt.Errorf("-t %s: sizes are 1, 2, 4 and 8", s)
		}
		return format{kind: s[0], size: size, width: widthFor(s[0], size)}, nil
	}
	return format{}, fmt.Errorf("-t %s: unknown type", s)
}

// widthFor is the printed width of one field, as GNU lays it out.
func widthFor(kind byte, size int) int {
	switch kind {
	case 'd', 'u':
		switch size {
		case 1:
			return 4
		case 2:
			return 6
		case 4:
			return 11
		default:
			return 20
		}
	case 'o':
		switch size {
		case 1:
			return 3
		case 2:
			return 6
		case 4:
			return 11
		default:
			return 22
		}
	case 'x':
		switch size {
		case 1:
			return 2
		case 2:
			return 4
		case 4:
			return 8
		default:
			return 16
		}
	}
	return 0
}

// readLimit reads at most limit bytes (all when limit is negative).
func readLimit(r io.Reader, limit int64) ([]byte, error) {
	if limit < 0 {
		return io.ReadAll(r)
	}
	return io.ReadAll(io.LimitReader(r, limit))
}

// dump writes the lines: one line per block and format, the address in front of
// each unless the radix is n, a "*" where a block repeats the one before, and the
// final address - the shape od has had since Version 7.
func dump(w io.Writer, data []byte, radix rune, formats []format, squash bool, base int64) {
	const perLine = 16
	var prev []byte
	starred := false
	for off := 0; off < len(data); off += perLine {
		end := off + perLine
		if end > len(data) {
			end = len(data)
		}
		chunk := data[off:end]
		if squash && prev != nil && string(prev) == string(chunk) {
			if !starred {
				fmt.Fprintln(w, "*")
				starred = true
			}
			continue
		}
		starred = false
		for _, f := range formats {
			var sb strings.Builder
			if radix != 'n' {
				sb.WriteString(addr(base+int64(off), radix))
			}
			sb.WriteString(fields(chunk, f))
			fmt.Fprintln(w, sb.String())
		}
		prev = chunk
	}
	if radix != 'n' {
		fmt.Fprintln(w, addr(base+int64(len(data)), radix))
	}
}

// addr renders one address in the chosen radix.
func addr(n int64, radix rune) string {
	switch radix {
	case 'd':
		return fmt.Sprintf("%07d", n)
	case 'x':
		return fmt.Sprintf("%07x", n)
	case 'n':
		return ""
	}
	return fmt.Sprintf("%07o", n)
}

func pad(s string, width int) string {
	for len(s) < width {
		s = " " + s
	}
	return s
}

// fields renders one block for one format.
func fields(chunk []byte, f format) string {
	var sb strings.Builder
	switch f.kind {
	case 'a':
		for _, b := range chunk {
			sb.WriteString(" ")
			sb.WriteString(namedChar(b))
		}
		return sb.String()
	case 'c':
		for _, b := range chunk {
			sb.WriteString(" ")
			sb.WriteString(cField(b))
		}
		return sb.String()
	}
	for i := 0; i < len(chunk); i += f.size {
		part := chunk[i:]
		if len(part) < f.size {
			part = append(append([]byte(nil), part...), make([]byte, f.size-len(part))...)
		}
		sb.WriteString(" ")
		sb.WriteString(number(part[:f.size], f))
	}
	return sb.String()
}

// number renders one value of size bytes little endian, as od does on x86.
func number(b []byte, f format) string {
	switch f.kind {
	case 'f':
		if f.size == 4 {
			v := math.Float32frombits(binary.LittleEndian.Uint32(b))
			return pad(strconv.FormatFloat(float64(v), 'e', 7, 32), 15)
		}
		v := math.Float64frombits(binary.LittleEndian.Uint64(b))
		return pad(strconv.FormatFloat(v, 'e', 15, 64), 24)
	}
	var v uint64
	for i := f.size - 1; i >= 0; i-- {
		v = v<<8 | uint64(b[i])
	}
	// od pads numbers with zeros in hex and octal (0x0a is "0a", not " a") and
	// with spaces in the decimal types, which is what scripts parse.
	switch f.kind {
	case 'x':
		return fmt.Sprintf("%0*x", f.width, v)
	case 'o':
		return fmt.Sprintf("%0*o", f.width, v)
	case 'u':
		return pad(strconv.FormatUint(v, 10), f.width)
	default: // d, signed
		return pad(strconv.FormatInt(signed(v, f.size), 10), f.width)
	}
}

// signed interprets v as a signed integer of size bytes.
func signed(v uint64, size int) int64 {
	switch size {
	case 1:
		return int64(int8(v))
	case 2:
		return int64(int16(v))
	case 4:
		return int64(int32(v))
	}
	return int64(v)
}

// cField is one character the way od -c prints it.
func cField(b byte) string {
	switch b {
	case 0:
		return `\0`
	case 7:
		return `\a`
	case 8:
		return `\b`
	case 9:
		return `\t`
	case 10:
		return `\n`
	case 11:
		return `\v`
	case 12:
		return `\f`
	case 13:
		return `\r`
	case '\\':
		return `\\`
	}
	if b >= 0x20 && b < 0x7f {
		return string(rune(b))
	}
	return pad(strconv.FormatUint(uint64(b), 8), 3)
}

// namedChar is one character for -t a: the ASCII names, else an escaped octet.
func namedChar(b byte) string {
	names := map[byte]string{
		0: "nul", 7: "bel", 8: "bs", 9: "ht", 10: "nl", 11: "vt", 12: "ff",
		13: "cr", 27: "esc", 32: "sp", 127: "del",
	}
	if n, ok := names[b]; ok {
		return n
	}
	if b >= 0x20 && b < 0x7f {
		return string(rune(b))
	}
	return pad(strconv.FormatUint(uint64(b), 8), 3)
}
