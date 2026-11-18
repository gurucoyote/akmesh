package iconv

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// charset is one supported character set, with a decoder to UTF-8 and an
// encoder back. The set is deliberately small: the cases an agent meets are
// Windows files (CP1252), old Latin-1 sources, ASCII, and UTF-16 from Windows
// tools. Adding a table is cheap; see docs/iconv-und-jq.md for the sizes of the
// alternatives.
type charset struct {
	name   string
	alias  []string
	decode func([]byte) (string, error)
	encode func(string) ([]byte, error)
}

// cp1252High maps the range 0x80..0x9F of Windows-1252. A zero entry is
// undefined in the code page.
var cp1252High = [32]rune{
	0x20AC, 0, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017D, 0,
	0, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0, 0x017E, 0x0178,
}

// latin9Diff replaces eight positions of Latin-1 with their ISO-8859-15
// meaning.
var latin9Diff = map[byte]rune{
	0xA4: 0x20AC, 0xA6: 0x0160, 0xA8: 0x0161, 0xB4: 0x017D,
	0xB8: 0x017E, 0xBC: 0x0152, 0xBD: 0x0153, 0xBE: 0x0178,
}

var charsets = []charset{
	{name: "UTF-8", alias: []string{"UTF8", "UNICODE-1-1-UTF-8"}, decode: decodeUTF8, encode: encodeUTF8},
	{name: "ASCII", alias: []string{"US-ASCII", "ANSI_X3.4-1968", "646"}, decode: decodeASCII, encode: encodeASCII},
	{name: "ISO-8859-1", alias: []string{"LATIN1", "L1", "ISO8859-1", "ISO88591"}, decode: decodeLatin1, encode: encodeSingleByte(latin1Table())},
	{name: "ISO-8859-15", alias: []string{"LATIN9", "L9", "ISO8859-15"}, decode: decodeLatin9, encode: encodeSingleByte(latin9Table())},
	{name: "WINDOWS-1252", alias: []string{"CP1252", "WINDOWS1252", "MS-ANSI"}, decode: decodeCP1252, encode: encodeSingleByte(cp1252Table())},
	{name: "UTF-16LE", alias: []string{"UTF16LE", "UCS-2LE"}, decode: decodeUTF16LE, encode: encodeUTF16LE},
	{name: "UTF-16BE", alias: []string{"UTF16BE", "UCS-2BE"}, decode: decodeUTF16BE, encode: encodeUTF16BE},
	{name: "UTF-16", alias: []string{"UTF16"}, decode: decodeUTF16Auto, encode: encodeUTF16BOM},
}

// latin1Table maps the 256 single byte values to their runes.
func latin1Table() []rune {
	table := make([]rune, 256)
	for i := range table {
		table[i] = rune(i)
	}
	return table
}

func latin9Table() []rune {
	table := latin1Table()
	for b, r := range latin9Diff {
		table[b] = r
	}
	return table
}

func cp1252Table() []rune {
	table := latin1Table()
	for i, r := range cp1252High {
		if r == 0 {
			// Undefined in the code page, marked so the encoder skips it.
			table[0x80+i] = -1
			continue
		}
		table[0x80+i] = r
	}
	return table
}

func lookupCharset(name string) (*charset, error) {
	clean := strings.ToUpper(strings.TrimSpace(name))
	// ignore the //TRANSLIT and //IGNORE suffixes; the behaviour is the same
	if idx := strings.Index(clean, "//"); idx >= 0 {
		clean = clean[:idx]
	}
	for i := range charsets {
		cs := &charsets[i]
		if cs.name == clean {
			return cs, nil
		}
		for _, a := range cs.alias {
			if a == clean {
				return cs, nil
			}
		}
	}
	return nil, fmt.Errorf("unsupported character set %q", name)
}

func decodeUTF8(data []byte) (string, error) {
	if !utf8.Valid(data) {
		return "", fmt.Errorf("invalid UTF-8 byte sequence")
	}
	return string(data), nil
}

func encodeUTF8(s string) ([]byte, error) { return []byte(s), nil }

func decodeASCII(data []byte) (string, error) {
	runes := make([]rune, len(data))
	for i, b := range data {
		if b > 0x7F {
			return "", fmt.Errorf("byte %#x is not ASCII", b)
		}
		runes[i] = rune(b)
	}
	return string(runes), nil
}

func encodeASCII(s string) ([]byte, error) {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 0x7F {
			return nil, fmt.Errorf("character %q is not ASCII", r)
		}
		out = append(out, byte(r))
	}
	return out, nil
}

func decodeLatin1(data []byte) (string, error) {
	runes := make([]rune, len(data))
	for i, b := range data {
		runes[i] = rune(b)
	}
	return string(runes), nil
}

func decodeLatin9(data []byte) (string, error) {
	runes := make([]rune, len(data))
	table := latin9Table()
	for i, b := range data {
		runes[i] = table[b]
	}
	return string(runes), nil
}

func decodeCP1252(data []byte) (string, error) {
	runes := make([]rune, 0, len(data))
	for _, b := range data {
		if b >= 0x80 && b < 0xA0 {
			r := cp1252High[b-0x80]
			if r == 0 {
				return "", fmt.Errorf("byte %#x is undefined in Windows-1252", b)
			}
			runes = append(runes, r)
			continue
		}
		runes = append(runes, rune(b))
	}
	return string(runes), nil
}

// encodeSingleByte builds an encoder from a 256 entry table.
func encodeSingleByte(table []rune) func(string) ([]byte, error) {
	index := make(map[rune]byte, 256)
	for b, r := range table {
		if r < 0 {
			continue
		}
		if _, ok := index[r]; ok {
			continue
		}
		index[r] = byte(b)
	}
	return func(s string) ([]byte, error) {
		out := make([]byte, 0, len(s))
		for _, r := range s {
			b, ok := index[r]
			if !ok {
				return nil, fmt.Errorf("character %q is not representable", r)
			}
			out = append(out, b)
		}
		return out, nil
	}
}

func decodeUTF16LE(data []byte) (string, error) {
	return decodeUTF16(data, false)
}

func decodeUTF16BE(data []byte) (string, error) {
	return decodeUTF16(data, true)
}

// decodeUTF16Auto honours a byte order mark; without one it assumes big endian,
// which is what GNU iconv does.
func decodeUTF16Auto(data []byte) (string, error) {
	bigEndian := true
	if len(data) >= 2 {
		switch {
		case data[0] == 0xFF && data[1] == 0xFE:
			bigEndian = false
			data = data[2:]
		case data[0] == 0xFE && data[1] == 0xFF:
			data = data[2:]
		}
	}
	return decodeUTF16(data, bigEndian)
}

func decodeUTF16(data []byte, bigEndian bool) (string, error) {
	if len(data)%2 != 0 {
		return "", fmt.Errorf("odd number of bytes for UTF-16")
	}
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		if bigEndian {
			units = append(units, uint16(data[i])<<8|uint16(data[i+1]))
		} else {
			units = append(units, uint16(data[i+1])<<8|uint16(data[i]))
		}
	}
	return string(utf16.Decode(units)), nil
}

func encodeUTF16LE(s string) ([]byte, error) { return encodeUTF16(s, false), nil }
func encodeUTF16BE(s string) ([]byte, error) { return encodeUTF16(s, true), nil }

func encodeUTF16BOM(s string) ([]byte, error) {
	return append([]byte{0xFE, 0xFF}, encodeUTF16(s, true)...), nil
}

func encodeUTF16(s string, bigEndian bool) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 0, len(units)*2)
	for _, u := range units {
		if bigEndian {
			out = append(out, byte(u>>8), byte(u))
		} else {
			out = append(out, byte(u), byte(u>>8))
		}
	}
	return out
}

// runIconv converts between the supported character sets:
//
//	iconv -f FROM -t TO [FILE...]
//
// Output goes to stdout; unrepresentable characters are an error unless -c is
// given, in which case they are dropped.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	from, to := "", ""
	ignore := false
	list := false
	var files []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-l" || a == "--list":
			list = true
		case a == "-c":
			ignore = true
		case a == "-f" || a == "--from-code":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "iconv: -f needs a character set")
				return 2
			}
			i++
			from = args[i]
		case strings.HasPrefix(a, "--from-code="):
			from = strings.TrimPrefix(a, "--from-code=")
		case a == "-t" || a == "--to-code":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "iconv: -t needs a character set")
				return 2
			}
			i++
			to = args[i]
		case strings.HasPrefix(a, "--to-code="):
			to = strings.TrimPrefix(a, "--to-code=")
		case strings.HasPrefix(a, "-") && a != "-":
			fmt.Fprintf(stderr, "iconv: unknown option %q\n", a)
			return 2
		default:
			files = append(files, a)
		}
	}
	if list {
		for _, cs := range charsets {
			fmt.Fprintln(stdout, cs.name)
		}
		return 0
	}
	if from == "" || to == "" {
		fmt.Fprintln(stderr, "usage: iconv -f FROM -t TO [FILE...]")
		return 2
	}
	src, err := lookupCharset(from)
	if err != nil {
		fmt.Fprintf(stderr, "iconv: %v\n", err)
		return 2
	}
	dst, err := lookupCharset(to)
	if err != nil {
		fmt.Fprintf(stderr, "iconv: %v\n", err)
		return 2
	}
	if len(files) == 0 {
		files = []string{"-"}
	}
	code := 0
	for _, name := range files {
		var data []byte
		if name == "-" {
			data, err = io.ReadAll(stdin)
		} else {
			data, err = os.ReadFile(name)
		}
		if err != nil {
			fmt.Fprintf(stderr, "iconv: %v\n", err)
			code = 1
			continue
		}
		text, err := src.decode(data)
		if err != nil {
			fmt.Fprintf(stderr, "iconv: %s: %v\n", name, err)
			code = 1
			continue
		}
		if ignore {
			text = dropUnrepresentable(text, dst)
		}
		out, err := dst.encode(text)
		if err != nil {
			fmt.Fprintf(stderr, "iconv: %s: %v\n", name, err)
			code = 1
			continue
		}
		if _, err := stdout.Write(out); err != nil {
			fmt.Fprintf(stderr, "iconv: %v\n", err)
			return 1
		}
	}
	return code
}

// dropUnrepresentable removes the characters the target cannot hold, which is
// what iconv -c does.
func dropUnrepresentable(s string, dst *charset) string {
	var b strings.Builder
	for _, r := range s {
		if _, err := dst.encode(string(r)); err != nil {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
