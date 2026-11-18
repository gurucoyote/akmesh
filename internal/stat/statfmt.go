package stat

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// statFields is everything an agent asks stat for, gathered once per file.
// The numbers that Windows cannot provide stay zero; the text fields say
// "unknown" rather than inventing a value.
type statFields struct {
	name  string
	info  os.FileInfo
	extra fileStatExtra
}

// expandStatFormat replaces the % directives GNU stat knows. It walks the
// string once so that "%%" works and a directive is never mistaken for text.
func expandStatFormat(format string, d statFields) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' || i+1 >= len(format) {
			b.WriteByte(c)
			continue
		}
		i++
		directive := format[i]
		// Skip width and precision, keep the directive itself.
		for i+1 < len(format) && (format[i+1] >= '0' && format[i+1] <= '9' || format[i+1] == '.' || format[i+1] == '-') {
			i++
		}
		b.WriteString(statDirective(directive, d))
	}
	return b.String()
}

func statDirective(c byte, d statFields) string {
	info, ex := d.info, d.extra
	perm := info.Mode().Perm()
	switch c {
	case '%':
		return "%"
	case 'n':
		return d.name
	case 'N':
		return "'" + d.name + "'"
	case 's':
		return strconv.FormatInt(info.Size(), 10)
	case 'a':
		return strconv.FormatUint(uint64(perm), 8)
	case 'A':
		return info.Mode().String()
	case 'f':
		return strconv.FormatUint(uint64(rawMode(info)), 16)
	case 'F':
		return fileTypeOf(info)
	case 'i':
		return strconv.FormatUint(ex.Inode, 10)
	case 'h':
		return strconv.FormatUint(ex.Links, 10)
	case 'u':
		return strconv.FormatUint(ex.UID, 10)
	case 'g':
		return strconv.FormatUint(ex.GID, 10)
	case 'U':
		return userName(ex.UID)
	case 'G':
		return groupName(ex.GID)
	case 'b':
		return strconv.FormatUint(ex.Blocks, 10)
	case 'B':
		return "512"
	case 'o':
		return strconv.FormatUint(ex.IOBlock, 10)
	case 'd':
		return strconv.FormatUint(ex.Device, 10)
	case 'D':
		return strconv.FormatUint(ex.Device, 16)
	case 't':
		return strconv.FormatUint(ex.RdevMajor, 16)
	case 'T':
		return strconv.FormatUint(ex.RdevMinor, 16)
	case 'x':
		return ex.Atime.Format("2006-01-02 15:04:05.000000000 -0700")
	case 'X':
		return strconv.FormatInt(ex.Atime.Unix(), 10)
	case 'y':
		return info.ModTime().Format("2006-01-02 15:04:05.000000000 -0700")
	case 'Y':
		return strconv.FormatInt(info.ModTime().Unix(), 10)
	case 'z':
		return ex.Ctime.Format("2006-01-02 15:04:05.000000000 -0700")
	case 'Z':
		return strconv.FormatInt(ex.Ctime.Unix(), 10)
	case 'W':
		return "0"
	case 'm':
		return ""
	default:
		return "%" + string(c)
	}
}

// rawMode is the unix st_mode word; on Windows only the portable bits exist.
func rawMode(info os.FileInfo) uint32 {
	mode := uint32(info.Mode().Perm())
	if info.IsDir() {
		mode |= 0o040000
	} else if info.Mode()&os.ModeSymlink != 0 {
		mode |= 0o120000
	} else {
		mode |= 0o100000
	}
	return mode
}

// terseStat is GNU stat -t: one line, space separated, no labels.
func terseStat(d statFields) string {
	fields := []string{
		d.name,
		strconv.FormatInt(d.info.Size(), 10),
		strconv.FormatUint(d.extra.Blocks, 10),
		// GNU stat -t prints %f and %D in hex, not decimal.
		strconv.FormatUint(uint64(rawMode(d.info)), 16),
		strconv.FormatUint(d.extra.UID, 10),
		strconv.FormatUint(d.extra.GID, 10),
		strconv.FormatUint(d.extra.Device, 16),
		strconv.FormatUint(d.extra.Inode, 10),
		strconv.FormatUint(d.extra.Links, 10),
		strconv.FormatUint(d.extra.RdevMajor, 16),
		strconv.FormatUint(d.extra.RdevMinor, 16),
		strconv.FormatInt(d.extra.Atime.Unix(), 10),
		strconv.FormatInt(d.info.ModTime().Unix(), 10),
		strconv.FormatInt(d.extra.Ctime.Unix(), 10),
		"0",
		strconv.FormatUint(d.extra.IOBlock, 10),
	}
	return strings.Join(fields, " ") + "\n"
}

// statTimeFormat is the timestamp shape GNU prints in the default block: the
// nanoseconds and the offset are part of it, an agent may parse either.
const statTimeFormat = "2006-01-02 15:04:05.000000000 -0700"

// statTime prints a timestamp, or "-" when the platform does not know it. GNU
// prints "-" for an unknown birth time, so an agent checks the same thing.
func statTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(statTimeFormat)
}

// deviceParts splits a dev_t the way the Linux kernel does. It lives here
// because the default block shows the device number on every platform.
func deviceParts(dev uint64) (uint64, uint64) {
	major := (dev >> 8) & 0xfff
	major |= (dev >> 32) &^ 0xfff
	minor := (dev & 0xff) | ((dev >> 12) &^ 0xff)
	return major, minor
}

// defaultStat is what stat prints without -c: the block GNU prints, with the
// labels an agent looks for. The birth time stays "-" because no target of ours
// reports it.
func defaultStat(d statFields) string {
	info, ex := d.info, d.extra
	major, minor := deviceParts(ex.Device)
	var b strings.Builder
	fmt.Fprintf(&b, "  File: %s\n", d.name)
	fmt.Fprintf(&b, "  Size: %-10d\tBlocks: %-10d IO Block: %-6d %s\n",
		info.Size(), ex.Blocks, ex.IOBlock, fileTypeOf(info))
	fmt.Fprintf(&b, "Device: %d,%d\tInode: %-10d  Links: %d\n", major, minor, ex.Inode, ex.Links)
	fmt.Fprintf(&b, "Access: (%04o/%s)  Uid: (%5d/%8s)   Gid: (%5d/%8s)\n",
		info.Mode().Perm(), info.Mode().String(), ex.UID, userName(ex.UID), ex.GID, groupName(ex.GID))
	fmt.Fprintf(&b, "Access: %s\n", statTime(ex.Atime))
	fmt.Fprintf(&b, "Modify: %s\n", info.ModTime().Format(statTimeFormat))
	fmt.Fprintf(&b, "Change: %s\n", statTime(ex.Ctime))
	b.WriteString(" Birth: -\n")
	return b.String()
}

// expandFsFormat fills the directives of stat -f. An unknown directive prints
// "?" as GNU does, so a typo in a format is visible instead of silent.
func expandFsFormat(format string, fs map[byte]string) string {
	var b strings.Builder
	for i := 0; i < len(format); i++ {
		c := format[i]
		if c != '%' || i+1 >= len(format) {
			b.WriteByte(c)
			continue
		}
		i++
		if format[i] == '%' {
			b.WriteByte('%')
			continue
		}
		if v, ok := fs[format[i]]; ok {
			b.WriteString(v)
		} else {
			b.WriteString("?")
		}
	}
	return b.String()
}

// defaultFsStat is the block GNU stat -f prints without -c, reduced to the
// lines an agent reads: name, type, block size and the free space.
func defaultFsStat(name string, fs map[byte]string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "  File: %q\n", name)
	fmt.Fprintf(&b, "    ID: %s Namelen: %s     Type: %s\n", fsID(fs), fs['l'], fs['T'])
	fmt.Fprintf(&b, "Block size: %s       Fundamental block size: %s\n", fs['s'], fs['S'])
	fmt.Fprintf(&b, "Blocks: Total: %s      Free: %s      Available: %s\n", fs['b'], fs['f'], fs['a'])
	fmt.Fprintf(&b, "Inodes: Total: %s      Free: %s\n", fs['c'], fs['d'])
	return b.String()
}

// fsID prints the file system id in hex, as GNU does. The value comes from the
// kernel, so the digits differ from host to host; the shape does not.
func fsID(fs map[byte]string) string {
	id := fs['i']
	if id == "" {
		return "?"
	}
	if n, err := strconv.ParseUint(id, 10, 64); err == nil {
		return strconv.FormatUint(n, 16)
	}
	return id
}

// terseFsStat is stat -f -t: one line in the field order GNU uses, so an agent
// that splits on spaces reads the same positions.
func terseFsStat(name string, fs map[byte]string) string {
	fields := []string{name, fsID(fs), fs['l'], fs['T'], fs['s'], fs['S'],
		fs['b'], fs['f'], fs['a'], fs['c'], fs['d']}
	return strings.Join(fields, " ") + "\n"
}

// unescapePrintf turns the backslash escapes GNU accepts in --printf into the
// characters they mean. -c does not do this, which is why the caller only
// passes a --printf format here.
func unescapePrintf(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '\\':
			b.WriteByte('\\')
		case '"':
			b.WriteByte('"')
		default:
			// Unknown escape: GNU keeps it verbatim.
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
