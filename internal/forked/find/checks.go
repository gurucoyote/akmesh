// Local addition, no upstream source: akmesh's own find tests a file against the pattern kinds.

package find

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// atoiNonNegative parses a depth value.
func atoiNonNegative(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("not a number")
	}
	return n, nil
}

// pathMatch is filepath.Match with the Unix separator, so the same pattern
// works on Windows.
func pathMatch(pattern, name string) (bool, error) {
	return filepath.Match(pattern, name)
}

func compileRegex(pattern string) (*regexp.Regexp, error) {
	return regexp.Compile(pattern)
}

func statTime(file string) (time.Time, error) {
	info, err := os.Stat(file)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// typeCheck builds the test for -type. A comma list means "any of these", as
// GNU allows.
func typeCheck(letters string) (func(os.FileInfo) bool, error) {
	var checks []func(os.FileInfo) bool
	for one := range strings.SplitSeq(letters, ",") {
		check, err := singleType(one)
		if err != nil {
			return nil, err
		}
		checks = append(checks, check)
	}
	return func(info os.FileInfo) bool {
		for _, c := range checks {
			if c(info) {
				return true
			}
		}
		return false
	}, nil
}

func singleType(letter string) (func(os.FileInfo) bool, error) {
	mode := info0Mode()
	switch letter {
	case "f":
		return func(info os.FileInfo) bool { return info.Mode().IsRegular() }, nil
	case "d":
		return func(info os.FileInfo) bool { return info.IsDir() }, nil
	case "l":
		return func(info os.FileInfo) bool { return info.Mode()&os.ModeSymlink != 0 }, nil
	case "s":
		return func(info os.FileInfo) bool { return info.Mode()&os.ModeSocket != 0 }, nil
	case "p":
		return func(info os.FileInfo) bool { return info.Mode()&os.ModeNamedPipe != 0 }, nil
	case "b":
		return func(info os.FileInfo) bool { return info.Mode()&os.ModeDevice != 0 && mode&os.ModeCharDevice == 0 }, nil
	case "c":
		return func(info os.FileInfo) bool { return info.Mode()&os.ModeCharDevice != 0 }, nil
	}
	return nil, fmt.Errorf("not a valid file type: %q", letter)
}

// info0Mode keeps the compiler from folding the char/block test above; it is
// zero and carries no meaning.
func info0Mode() os.FileMode { return 0 }

// numberCompare parses GNU's "+N", "-N" and "N" and returns a test for a value
// already in the right unit.
func numberCompare(spec string) (func(int64) bool, error) {
	sign := byte(0)
	body := spec
	if len(body) > 0 && (body[0] == '+' || body[0] == '-') {
		sign = body[0]
		body = body[1:]
	}
	n, err := strconv.ParseInt(body, 10, 64)
	if err != nil || n < 0 {
		return nil, fmt.Errorf("invalid number %q", spec)
	}
	switch sign {
	case '+':
		return func(v int64) bool { return v > n }, nil
	case '-':
		return func(v int64) bool { return v < n }, nil
	}
	return func(v int64) bool { return v == n }, nil
}

// sizeCheck builds the test for -size. The unit letters are GNU's: c bytes,
// w two-byte words, b 512-byte blocks (the default), k KiB, M MiB, G GiB. A
// file is measured in whole units, rounded up.
func sizeCheck(spec string) (func(os.FileInfo) bool, error) {
	if spec == "" {
		return nil, fmt.Errorf("-size needs a value")
	}
	unit := int64(512)
	body := spec
	switch body[len(body)-1] {
	case 'c':
		unit, body = 1, body[:len(body)-1]
	case 'w':
		unit, body = 2, body[:len(body)-1]
	case 'b':
		unit, body = 512, body[:len(body)-1]
	case 'k':
		unit, body = 1024, body[:len(body)-1]
	case 'M':
		unit, body = 1024*1024, body[:len(body)-1]
	case 'G':
		unit, body = 1024*1024*1024, body[:len(body)-1]
	}
	cmp, err := numberCompare(body)
	if err != nil {
		return nil, fmt.Errorf("invalid -size %q", spec)
	}
	return func(info os.FileInfo) bool {
		size := info.Size()
		units := size / unit
		if size%unit != 0 {
			units++
		}
		return cmp(units)
	}, nil
}

// ownerCheck builds the test for -user and -group.
func ownerCheck(isUser bool, value string) (func(os.FileInfo) bool, error) {
	want, err := resolveID(isUser, value)
	if err != nil {
		return nil, err
	}
	return func(info os.FileInfo) bool {
		id, ok := idOf(info, isUser)
		return ok && id == want
	}, nil
}

// resolveID accepts a name or a number, as GNU does.
func resolveID(isUser bool, value string) (uint32, error) {
	if n, err := strconv.ParseUint(value, 10, 32); err == nil {
		return uint32(n), nil
	}
	if !idsAvailable() {
		return 0, fmt.Errorf("%s: users and groups are not available on this platform", value)
	}
	if isUser {
		u, err := user.Lookup(value)
		if err != nil {
			return 0, fmt.Errorf("%s: no such user", value)
		}
		n, err := strconv.ParseUint(u.Uid, 10, 32)
		if err != nil {
			return 0, err
		}
		return uint32(n), nil
	}
	g, err := user.LookupGroup(value)
	if err != nil {
		return 0, fmt.Errorf("%s: no such group", value)
	}
	n, err := strconv.ParseUint(g.Gid, 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(n), nil
}

// permCheck builds the test for -perm and the older -mode. An octal mode means
// "exactly these bits", with a leading "-" every bit has to be set, with a
// leading "/" any of them; a symbolic mode like -u+x works too.
func permCheck(kind, spec string) (func(os.FileInfo) bool, error) {
	mode, mask, any, err := parsePerm(spec)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", kind, err)
	}
	return func(info os.FileInfo) bool {
		got := uint32(info.Mode().Perm())
		switch {
		case any:
			return got&mode != 0
		case mask:
			return got&mode == mode
		default:
			return got == mode
		}
	}, nil
}

// parsePerm returns the bits, whether all of them have to be set, and whether
// any of them is enough.
func parsePerm(spec string) (mode uint32, all bool, any bool, err error) {
	switch {
	case strings.HasPrefix(spec, "-"):
		all, spec = true, spec[1:]
	case strings.HasPrefix(spec, "/"):
		any, spec = true, spec[1:]
	}
	if spec == "" {
		return 0, false, false, fmt.Errorf("empty mode")
	}
	if n, perr := strconv.ParseUint(spec, 8, 32); perr == nil {
		return uint32(n), all, any, nil
	}
	bits, serr := symbolicPerm(spec)
	if serr != nil {
		return 0, false, false, serr
	}
	return bits, all, any, nil
}

// symbolicPerm converts a mode like "u+x", "a=rw" or "go-w" to bits.
func symbolicPerm(spec string) (uint32, error) {
	var mode uint32
	for clause := range strings.SplitSeq(spec, ",") {
		i := strings.IndexAny(clause, "+-=")
		if i < 0 {
			return 0, fmt.Errorf("invalid mode %q", spec)
		}
		who, op, perms := clause[:i], clause[i], clause[i+1:]
		var whoBits uint32
		if who == "" {
			who = "a"
		}
		for _, c := range who {
			switch c {
			case 'u':
				whoBits |= 0o700
			case 'g':
				whoBits |= 0o070
			case 'o':
				whoBits |= 0o007
			case 'a':
				whoBits |= 0o777
			default:
				return 0, fmt.Errorf("invalid mode %q", spec)
			}
		}
		var bits uint32
		for _, c := range perms {
			switch c {
			case 'r':
				bits |= 0o444
			case 'w':
				bits |= 0o222
			case 'x', 'X':
				bits |= 0o111
			case 's', 't':
				// setuid, setgid and sticky are not part of the permission
				// bits find compares here; ignore them rather than fail.
			default:
				return 0, fmt.Errorf("invalid mode %q", spec)
			}
		}
		bits &= whoBits
		switch op {
		case '+':
			mode |= bits
		case '-':
			mode &^= bits
		case '=':
			mode = (mode &^ whoBits) | bits
		}
	}
	return mode, nil
}

// checkPrintf rejects a directive this find does not know, so a typo shows up
// as a usage error instead of printing rubbish.
func checkPrintf(format string) error {
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		if i+1 >= len(format) {
			return fmt.Errorf("-printf: %% at the end of the format")
		}
		if !strings.ContainsRune("pfysmdMhly%", rune(format[i+1])) {
			return fmt.Errorf("-printf: unknown directive %%%c", format[i+1])
		}
		i++
	}
	return nil
}

// unescapeFormat turns the backslash escapes GNU honours in a -printf format
// into the characters they mean; a real newline reaches us as \n.
func unescapeFormat(format string) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '\\' || i+1 >= len(format) {
			b.WriteByte(format[i])
			continue
		}
		i++
		switch format[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '0':
			b.WriteByte(0)
		case '\\':
			b.WriteByte('\\')
		case 'a':
			b.WriteByte(7)
		case 'b':
			b.WriteByte(8)
		case 'f':
			b.WriteByte(12)
		case 'v':
			b.WriteByte(11)
		default:
			b.WriteByte('\\')
			b.WriteByte(format[i])
		}
	}
	return b.String()
}

// printfFormat expands GNU's -printf directives for one entry.
func printfFormat(format string, info os.FileInfo, path string) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 >= len(format) {
			b.WriteByte(format[i])
			continue
		}
		i++
		switch format[i] {
		case 'p':
			b.WriteString(path)
		case 'f':
			b.WriteString(filepath.Base(path))
		case 's':
			b.WriteString(strconv.FormatInt(info.Size(), 10))
		case 'y':
			b.WriteString(typeLetter(info))
		case 'm':
			b.WriteString(strconv.FormatUint(uint64(info.Mode().Perm()), 8))
		case 'M':
			b.WriteString(info.Mode().String())
		case 'd':
			b.WriteString(strconv.Itoa(strings.Count(path, "/")))
		case 'h':
			b.WriteString(filepath.Dir(path))
		case 'l':
			if target, err := os.Readlink(path); err == nil {
				b.WriteString(target)
			}
		case '%':
			b.WriteByte('%')
		}
	}
	return b.String()
}

// typeLetter is GNU's letter for the file type.
func typeLetter(info os.FileInfo) string {
	switch mode := info.Mode(); {
	case mode.IsRegular():
		return "f"
	case mode.IsDir():
		return "d"
	case mode&os.ModeSymlink != 0:
		return "l"
	case mode&os.ModeSocket != 0:
		return "s"
	case mode&os.ModeNamedPipe != 0:
		return "p"
	case mode&os.ModeCharDevice != 0:
		return "c"
	case mode&os.ModeDevice != 0:
		return "b"
	}
	return "?"
}
