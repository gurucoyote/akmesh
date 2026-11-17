package tools

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// runFile identifies a file from its first bytes, the way file(1) does for the
// formats an agent actually meets. It is not a full magic database: the table
// is short and readable, so that a wrong answer can be traced to a line.
func runFile(args []string, stdin *os.File, stdout, stderr *os.File) int {
	brief := false
	mime := false
	var files []string
	for _, a := range args {
		switch a {
		case "-b", "--brief":
			brief = true
		case "-i", "--mime", "--mime-type":
			mime = true
		case "-h", "--help":
			fmt.Fprintln(stdout, "usage: file [-b] [-i] FILE...")
			return 0
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				fmt.Fprintf(stderr, "file: unknown option %q\n", a)
				return 2
			}
			files = append(files, a)
		}
	}
	if len(files) == 0 {
		fmt.Fprintln(stderr, "file: missing operand")
		return 2
	}
	code := 0
	for _, name := range files {
		kind, mimeType, err := describePath(name)
		if err != nil {
			fmt.Fprintf(stderr, "file: %v\n", err)
			code = 1
			continue
		}
		switch {
		case mime:
			fmt.Fprintf(stdout, "%s\n", mimeType)
		case brief:
			fmt.Fprintf(stdout, "%s\n", kind)
		default:
			fmt.Fprintf(stdout, "%s: %s\n", name, kind)
		}
	}
	return code
}

func describePath(name string) (kind, mimeType string, err error) {
	info, err := os.Lstat(name)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return "directory", "inode/directory", nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(name)
		if err != nil {
			return "symbolic link", "inode/symlink", nil
		}
		return "symbolic link to " + target, "inode/symlink", nil
	}
	if !info.Mode().IsRegular() {
		return "special file", "application/octet-stream", nil
	}
	if info.Size() == 0 {
		return "empty", "application/x-empty", nil
	}
	f, err := os.Open(name)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	head := make([]byte, 8192)
	n, _ := f.Read(head)
	head = head[:n]
	return describeBytes(head)
}

// describeBytes classifies a buffer. The order matters: a signature beats any
// text heuristic.
func describeBytes(head []byte) (string, string, error) {
	type magic struct {
		prefix []byte
		kind   string
		mime   string
	}
	signatures := []magic{
		{[]byte{0x7F, 'E', 'L', 'F'}, "", "application/x-executable"},
		{[]byte("%PDF-"), "PDF document", "application/pdf"},
		{[]byte{0x89, 'P', 'N', 'G'}, "PNG image data", "image/png"},
		{[]byte{0xFF, 0xD8, 0xFF}, "JPEG image data", "image/jpeg"},
		{[]byte("GIF87a"), "GIF image data, version 87a", "image/gif"},
		{[]byte("GIF89a"), "GIF image data, version 89a", "image/gif"},
		{[]byte("BM"), "BMP image data", "image/bmp"},
		{[]byte("SQLite format 3\x00"), "SQLite 3.x database", "application/vnd.sqlite3"},
		{[]byte("PK\x03\x04"), "Zip archive data", "application/zip"},
		{[]byte{0x1F, 0x8B}, "gzip compressed data", "application/gzip"},
		{[]byte("BZh"), "bzip2 compressed data", "application/x-bzip2"},
		{[]byte{0xFD, '7', 'z', 'X', 'Z', 0x00}, "XZ compressed data", "application/x-xz"},
		{[]byte{0x28, 0xB5, 0x2F, 0xFD}, "Zstandard compressed data", "application/zstd"},
		{[]byte("7z\xBC\xAF\x27\x1C"), "7-zip archive data", "application/x-7z-compressed"},
		{[]byte("OggS"), "Ogg data", "application/ogg"},
		{[]byte("fLaC"), "FLAC audio bitstream data", "audio/flac"},
		{[]byte("RIFF"), "RIFF data", "application/octet-stream"},
		{[]byte("ID3"), "Audio file with ID3 tag", "audio/mpeg"},
		{[]byte{0xCA, 0xFE, 0xBA, 0xBE}, "compiled Java class data", "application/java-vm"},
		{[]byte{0x00, 'a', 's', 'm'}, "WebAssembly binary module", "application/wasm"},
		{[]byte("wOFF"), "Web Open Font Format", "font/woff"},
		{[]byte("wOF2"), "Web Open Font Format (Version 2)", "font/woff2"},
	}
	for _, m := range signatures {
		if bytes.HasPrefix(head, m.prefix) {
			if len(m.prefix) == 4 && bytes.Equal(m.prefix, []byte{0x7F, 'E', 'L', 'F'}) {
				return describeELF(head), m.mime, nil
			}
			if bytes.Equal(m.prefix, []byte("RIFF")) && len(head) >= 12 {
				switch string(head[8:12]) {
				case "WAVE":
					return "RIFF (little-endian) data, WAVE audio", "audio/wav", nil
				case "AVI ":
					return "RIFF (little-endian) data, AVI video", "video/x-msvideo", nil
				case "WEBP":
					return "RIFF (little-endian) data, Web/P image", "image/webp", nil
				}
			}
			return m.kind, m.mime, nil
		}
	}
	if len(head) >= 2 && head[0] == 'M' && head[1] == 'Z' {
		return describePE(head), "application/vnd.microsoft.portable-executable", nil
	}
	if len(head) >= 262 && string(head[257:262]) == "ustar" {
		return "POSIX tar archive", "application/x-tar", nil
	}
	if len(head) >= 12 && string(head[4:8]) == "ftyp" {
		return "ISO Media, MP4 base media", "video/mp4", nil
	}
	return describeText(head)
}

func describeELF(head []byte) string {
	if len(head) < 20 {
		return "ELF data"
	}
	class := "32-bit"
	if head[4] == 2 {
		class = "64-bit"
	}
	order := "MSB"
	if head[5] == 1 {
		order = "LSB"
	}
	var machine string
	if head[5] == 1 {
		machine = elfMachine(binary.LittleEndian.Uint16(head[18:20]))
	} else {
		machine = elfMachine(binary.BigEndian.Uint16(head[18:20]))
	}
	kind := "executable"
	if len(head) >= 18 {
		var eType uint16
		if head[5] == 1 {
			eType = binary.LittleEndian.Uint16(head[16:18])
		} else {
			eType = binary.BigEndian.Uint16(head[16:18])
		}
		switch eType {
		case 3:
			kind = "shared object"
		case 4:
			kind = "core file"
		case 1:
			kind = "relocatable"
		}
	}
	return fmt.Sprintf("ELF %s %s %s, %s", class, order, kind, machine)
}

func elfMachine(code uint16) string {
	switch code {
	case 3:
		return "Intel 80386"
	case 62:
		return "x86-64"
	case 183:
		return "ARM aarch64"
	case 40:
		return "ARM"
	case 243:
		return "RISC-V"
	case 21:
		return "PowerPC64"
	case 8:
		return "MIPS"
	default:
		return fmt.Sprintf("machine %d", code)
	}
}

func describePE(head []byte) string {
	if len(head) < 0x40 {
		return "MS-DOS executable"
	}
	offset := int(binary.LittleEndian.Uint32(head[0x3C:0x40]))
	if offset <= 0 || offset+6 > len(head) {
		return "MS-DOS executable"
	}
	if string(head[offset:offset+4]) != "PE\x00\x00" {
		return "MS-DOS executable"
	}
	machine := binary.LittleEndian.Uint16(head[offset+4 : offset+6])
	name := "unknown machine"
	switch machine {
	case 0x8664:
		name = "x86-64"
	case 0x14c:
		name = "Intel 80386"
	case 0xAA64:
		name = "ARM aarch64"
	}
	// The optional header magic distinguishes PE32 from PE32+.
	optOffset := offset + 24
	if optOffset+2 <= len(head) {
		magic := binary.LittleEndian.Uint16(head[optOffset : optOffset+2])
		switch magic {
		case 0x20B:
			return fmt.Sprintf("PE32+ executable (console) %s, for MS Windows", name)
		case 0x10B:
			return fmt.Sprintf("PE32 executable (console) %s, for MS Windows", name)
		}
	}
	return fmt.Sprintf("PE executable %s, for MS Windows", name)
}

// describeText decides between text in a known encoding and binary data. The
// rules are the ones file(1) uses in spirit: a NUL byte or a control character
// that is not whitespace means data.
func describeText(head []byte) (string, string, error) {
	switch {
	case bytes.HasPrefix(head, []byte{0xEF, 0xBB, 0xBF}):
		return "UTF-8 Unicode (with BOM) text" + lineEndingNote(head[3:]), "text/plain", nil
	case bytes.HasPrefix(head, []byte{0xFF, 0xFE}):
		return "Unicode text, UTF-16, little-endian with BOM", "text/plain", nil
	case bytes.HasPrefix(head, []byte{0xFE, 0xFF}):
		return "Unicode text, UTF-16, big-endian with BOM", "text/plain", nil
	}
	if hasControlBytes(head) {
		return "data", "application/octet-stream", nil
	}
	if utf8.Valid(head) {
		ascii := true
		for _, b := range head {
			if b > 0x7F {
				ascii = false
				break
			}
		}
		label := "UTF-8 Unicode text"
		if ascii {
			label = "ASCII text"
		}
		return label + lineEndingNote(head), "text/plain", nil
	}
	return "ISO-8859 text, non-UTF-8", "text/plain", nil
}

func lineEndingNote(data []byte) string {
	crcr := bytes.Count(data, []byte{'\r', '\n'})
	lf := bytes.Count(data, []byte{'\n'})
	switch {
	case crcr > 0 && crcr == lf:
		return ", with CRLF line terminators"
	case crcr > 0:
		return ", with CRLF and LF line terminators"
	}
	return ""
}

func hasControlBytes(data []byte) bool {
	for _, b := range data {
		if b == 0 {
			return true
		}
		if b < 0x20 {
			switch b {
			case '\n', '\r', '\t', '\v', '\f', 0x1B:
				continue
			}
			return true
		}
		if b == 0x7F {
			return true
		}
	}
	return false
}
