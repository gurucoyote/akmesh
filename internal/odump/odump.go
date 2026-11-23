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
	defaultType := true
	perLine := 16
	var limit, skip int64 = -1, 0
	var files []string
	// takeValue returns the value of an option: the rest of the word when it is
	// glued (-An, -tc) or the next argument.
	takeValue := func(name, rest string, i *int) (string, error) {
		if rest != "" {
			return rest, nil
		}
		*i++
		if *i >= len(args) {
			return "", fmt.Errorf("option requires an argument -- '%s'", name)
		}
		return args[*i], nil
	}
	badOpt := func(err error) int {
		fmt.Fprintf(stderr, "od: %v\n", err)
		return 1
	}
	addType := func(v string) error {
		// -t holds a list: "-t x1o1" is two types, as GNU reads it.
		for v != "" {
			f, used, err := parseTypePrefix(v)
			if err != nil {
				return err
			}
			if defaultType && len(formats) == 1 {
				formats = nil
				defaultType = false
			}
			formats = append(formats, f)
			v = v[used:]
		}
		return nil
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			files = append(files, args[i+1:]...)
			i = len(args)
		case a == "--help":
			return 0
		case strings.HasPrefix(a, "--"):
			return badOpt(fmt.Errorf("unrecognized option '%s'", a))
		case len(a) > 1 && a[0] == '-':
			// The short options cluster the way GNU reads them: -An -tc, -cv,
			// -w4, -cd. Without this every historical spelling (-c, -b, -x)
			// failed as an unknown option (measured 24.11.2026, acmesh-3qy).
			rest := a[1:]
			for rest != "" {
				c := rest[0]
				rest = rest[1:]
				if t, ok := shortTypes[c]; ok {
					if err := addType(t); err != nil {
						return badOpt(err)
					}
					continue
				}
				switch c {
				case 'v':
					squash = false
				case 'A':
					v, err := takeValue("A", rest, &i)
					if err != nil {
						return badOpt(err)
					}
					switch v {
					case "d", "o", "x", "n":
						radix = rune(v[0])
					default:
						return badOpt(fmt.Errorf("-A %s: unknown radix", v))
					}
					rest = ""
				case 't':
					v, err := takeValue("t", rest, &i)
					if err != nil {
						return badOpt(err)
					}
					if err := addType(v); err != nil {
						return badOpt(err)
					}
					rest = ""
				case 'N', 'j', 'w':
					v, err := takeValue(string(c), rest, &i)
					if err != nil {
						return badOpt(err)
					}
					n, err := parseCount(v)
					if err != nil {
						return badOpt(err)
					}
					switch c {
					case 'N':
						limit = n
					case 'j':
						skip = n
					case 'w':
						perLine = int(n)
					}
					rest = ""
				default:
					return badOpt(fmt.Errorf("invalid option -- '%c'", c))
				}
			}
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
	dump(stdout, data, radix, formats, squash, skip, perLine)
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

// shortTypes are the historical one letter formats, each a shorthand for one
// -t type: -c is -t c, -b is -t o1, -x is -t x2 and so on (GNU od's manual).
var shortTypes = map[byte]string{
	'a': "a",
	'b': "o1",
	'c': "c",
	'd': "u2",
	'o': "o2",
	's': "d2",
	'x': "x2",
}

// parseTypePrefix reads one type off the front of s and says how many bytes it
// took, so a list such as x1o1 can be read in one -t.
func parseTypePrefix(s string) (format, int, error) {
	if s == "" {
		return format{}, 0, fmt.Errorf("-t without a type")
	}
	switch kind := s[0]; kind {
	case 'a', 'c':
		return format{kind: kind, size: 1}, 1, nil
	case 'd', 'o', 'u', 'x', 'f':
		j := 1
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == 1 {
			return format{}, 0, fmt.Errorf("-t %c: a size is needed (1, 2, 4, 8)", kind)
		}
		size, err := strconv.Atoi(s[1:j])
		if err != nil {
			return format{}, 0, fmt.Errorf("-t %s: unknown size", s[:j])
		}
		if kind == 'f' && size != 4 && size != 8 {
			return format{}, 0, fmt.Errorf("-t %s: floats are 4 or 8 bytes", s[:j])
		}
		if kind != 'f' && size != 1 && size != 2 && size != 4 && size != 8 {
			return format{}, 0, fmt.Errorf("-t %s: sizes are 1, 2, 4 and 8", s[:j])
		}
		return format{kind: kind, size: size, width: widthFor(kind, size)}, j, nil
	}
	return format{}, 0, fmt.Errorf("-t %s: unknown type", s)
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
	case 'd':
		// Signed fields keep room for the sign; unsigned are one narrower
		// (measured against GNU od 9.7: 4/6/11/20 signed, 3/5/10/20 unsigned).
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
	case 'u':
		switch size {
		case 1:
			return 3
		case 2:
			return 5
		case 4:
			return 10
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
func dump(w io.Writer, data []byte, radix rune, formats []format, squash bool, base int64, perLine int) {
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
		widths := columnWidths(formats)
		for fi, f := range formats {
			var sb strings.Builder
			if radix != 'n' {
				if fi == 0 {
					sb.WriteString(addr(base+int64(off), radix))
				} else {
					// The address stands once per block; the later formats of
					// the same block are indented to its width, as GNU does.
					sb.WriteString(strings.Repeat(" ", len(addr(base+int64(off), radix))))
				}
			}
			for k, v := range rawFields(chunk, f) {
				width := fieldWidth(f)
				if k < len(widths) {
					width = widths[k]
				}
				sb.WriteString(" ")
				sb.WriteString(pad(v, width))
			}
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
		// GNU pads a hex address to six digits, an octal or decimal one to
		// seven (od -A x of eight bytes ends with "000008").
		return fmt.Sprintf("%06x", n)
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

// fieldWidth is the column width of one value of this format, as GNU counts it.
func fieldWidth(f format) int {
	if f.kind == 'a' || f.kind == 'c' {
		return 3
	}
	return f.width
}

// rawFields renders one block for one format, one string per value, without the
// separating blank and without column padding; the numbers keep the zero
// padding their radix asks for.
func rawFields(chunk []byte, f format) []string {
	out := make([]string, 0, len(chunk))
	switch f.kind {
	case 'a':
		for _, b := range chunk {
			out = append(out, namedChar(b))
		}
		return out
	case 'c':
		for _, b := range chunk {
			out = append(out, cField(b))
		}
		return out
	}
	for i := 0; i < len(chunk); i += f.size {
		part := chunk[i:]
		if len(part) < f.size {
			part = append(append([]byte(nil), part...), make([]byte, f.size-len(part))...)
		}
		out = append(out, strings.TrimLeft(number(part[:f.size], f), " "))
	}
	return out
}

// columnWidths holds, for every position in the line, the widest value any of
// the formats prints there. GNU lines the columns up across formats, so
// "-t x1o1" pads the hex bytes to the width of the octal ones.
func columnWidths(formats []format) []int {
	var widths []int
	for _, f := range formats {
		w := fieldWidth(f)
		for len(widths) < 16 {
			widths = append(widths, 0)
		}
		for i := range widths {
			if w > widths[i] {
				widths[i] = w
			}
		}
	}
	return widths
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
