package ls

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// widths holds the column widths of one listing. GNU pads each column to the
// widest value in the same listing, so -l output lines up.
type widths struct {
	inode, links, owner, group, size int
}

func (l lister) widthsOf(entries []entry) widths {
	var w widths
	for _, e := range entries {
		w.links = max(w.links, len(strconv.FormatUint(e.links(), 10)))
		w.owner = max(w.owner, len(l.ownerName(e)))
		w.group = max(w.group, len(l.groupName(e)))
		w.size = max(w.size, len(l.sizeText(e)))
		if l.o.inode {
			w.inode = max(w.inode, len(strconv.FormatUint(e.inode(), 10)))
		}
	}
	return w
}

// longText is the -l line: either inode, mode, links, owner, group, size, date,
// name. Widths are zero when there is nothing to align to.
func (l lister) longText(e entry, w widths) string {
	var b strings.Builder
	if l.o.inode {
		fmt.Fprintf(&b, "%-*d ", w.inode, e.inode())
	}
	b.WriteString(modeString(e.mode))
	b.WriteString(" ")
	fmt.Fprintf(&b, "%*d ", w.links, e.links())
	owner, group := l.ownerName(e), l.groupName(e)
	fmt.Fprintf(&b, "%-*s %-*s ", w.owner, owner, w.group, group)
	fmt.Fprintf(&b, "%*s ", w.size, l.sizeText(e))
	b.WriteString(l.timeText(l.sortTime(e)))
	b.WriteString(" ")
	b.WriteString(l.displayName(e))
	if e.mode&os.ModeSymlink != 0 {
		// GNU appends " -> target" for a symlink in the long form.
		if target, err := os.Readlink(e.path); err == nil {
			fmt.Fprintf(&b, " -> %s", target)
		}
	}
	return b.String()
}

// modeString is the ten-character mode: type letter, then rwx with the setuid,
// setgid and sticky variants.
func modeString(m os.FileMode) string {
	var b [10]byte
	switch {
	case m&os.ModeSymlink != 0:
		b[0] = 'l'
	case m.IsDir():
		b[0] = 'd'
	case m&os.ModeCharDevice != 0:
		b[0] = 'c'
	case m&os.ModeDevice != 0:
		b[0] = 'b'
	case m&os.ModeNamedPipe != 0:
		b[0] = 'p'
	case m&os.ModeSocket != 0:
		b[0] = 's'
	default:
		b[0] = '-'
	}
	const rwx = "rwxrwxrwx"
	for i := 0; i < 9; i++ {
		if m.Perm()&(1<<uint(8-i)) != 0 {
			b[i+1] = rwx[i]
		} else {
			b[i+1] = '-'
		}
	}
	// s/S in the execute position means "set, but not executable".
	if m&os.ModeSetuid != 0 {
		if b[3] == 'x' {
			b[3] = 's'
		} else {
			b[3] = 'S'
		}
	}
	if m&os.ModeSetgid != 0 {
		if b[6] == 'x' {
			b[6] = 's'
		} else {
			b[6] = 'S'
		}
	}
	if m&os.ModeSticky != 0 {
		if b[9] == 'x' {
			b[9] = 't'
		} else {
			b[9] = 'T'
		}
	}
	return string(b[:])
}

// sizeText is the size column: bytes, or GNU's binary human form with -h
// (1.0K, 234M) - one decimal below ten, whole numbers above, no space.
func (l lister) sizeText(e entry) string {
	if !l.o.human {
		return strconv.FormatInt(e.size, 10)
	}
	return humanSize(e.size)
}

func humanSize(n int64) string {
	if n < 1024 {
		return strconv.FormatInt(n, 10)
	}
	units := []string{"K", "M", "G", "T", "P", "E"}
	v := float64(n)
	i := -1
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if v < 10 {
		return fmt.Sprintf("%.1f%s", v, units[i])
	}
	return fmt.Sprintf("%.0f%s", v, units[i])
}

// timeText is GNU's date rule: within the last six months (and not more than an
// hour ahead) the time is shown, otherwise the year. The old u-root command
// dropped the year for old files and printed the current time instead.
func (l lister) timeText(t time.Time) string {
	sixMonths := 6 * 30 * 24 * time.Hour
	recent := t.After(l.now.Add(-sixMonths)) && t.Before(l.now.Add(time.Hour))
	if recent {
		return t.Format("Jan _2 15:04")
	}
	return t.Format("Jan _2  2006")
}

// sortTime is the time a listing sorts by: mtime, or atime/ctime with -u/-c.
func (l lister) sortTime(e entry) time.Time {
	switch {
	case l.o.byAccess:
		if t, ok := e.accessTime(); ok {
			return t
		}
	case l.o.byChange:
		if t, ok := e.changeTime(); ok {
			return t
		}
	}
	return e.mtime
}

// displayName applies -Q and -F, in that order: GNU quotes the name and puts the
// indicator outside the quote ("link"@).
func (l lister) displayName(e entry) string {
	name := e.name
	if l.o.quote {
		name = `"` + name + `"`
	}
	if l.o.classify {
		name += indicator(e.mode)
	}
	return name
}

func indicator(m os.FileMode) string {
	switch {
	case m.IsDir():
		return "/"
	case m&os.ModeSymlink != 0:
		return "@"
	case m&os.ModeNamedPipe != 0:
		return "|"
	case m&os.ModeSocket != 0:
		return "="
	case m&0111 != 0:
		return "*"
	}
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
