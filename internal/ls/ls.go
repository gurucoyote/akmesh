// Package ls lists directory contents the way GNU ls does, because an agent
// types what it knows: -l, -a, -lt, -R, -h.
//
// It is a re-implementation with the standard library, not a fork of u-root's
// ls. The upstream command printed "Jan 1 19:55" for a file from 2020, had no
// total line, wrote "L" for a symlink and refused -t, -r, -1, -i, -n, -A, -u and
// -c with exit 2 (acmesh-wx6). Fixing all of that inside the fork would have
// meant carrying 374 lines of u-root packages; writing it once costs less.
//
// What it cannot know it does not invent: on Windows there are no link counts
// and no POSIX inode numbers, so those columns read 1 and 0 rather than a
// plausible lie. The mode on Windows comes from winmode, the ACL record chmod
// writes (acmesh-m6x).
package ls

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"akmesh/internal/winmode"
)

type options struct {
	all       bool // -a: every entry, including . and ..
	almost    bool // -A: every entry except . and ..
	one       bool // -1: one per line
	directory bool // -d: the directory itself, not its contents
	classify  bool // -F: type indicator
	human     bool // -h: human readable sizes
	inode     bool // -i: inode numbers
	long      bool // -l: long form
	numeric   bool // -n: numeric uid and gid
	quote     bool // -Q: quoted names
	recurse   bool // -R: descend
	reverse   bool // -r: reverse the order
	bySize    bool // -S: sort by size, largest first
	byTime    bool // -t: sort by modification time, newest first
	byAccess  bool // -u: use access time
	byChange  bool // -c: use change time
}

// Run is the entry the akmesh binary calls.
func Run(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var o options
	var paths []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			paths = append(paths, args[i+1:]...)
			i = len(args)
		case a == "--help":
			fmt.Fprintln(stdout, "Usage: ls [OPTION]... [FILE]...")
			fmt.Fprintln(stdout, "List information about the FILEs, sorted by name.")
			fmt.Fprintln(stdout)
			fmt.Fprintln(stdout, "  -a  do not ignore entries starting with .")
			fmt.Fprintln(stdout, "  -A  do not list implied . and ..")
			fmt.Fprintln(stdout, "  -1  one entry per line")
			fmt.Fprintln(stdout, "  -d  list directories themselves, not their contents")
			fmt.Fprintln(stdout, "  -F  append indicator (one of */=@|)")
			fmt.Fprintln(stdout, "  -h  with -l, print sizes like 1K 234M 2G")
			fmt.Fprintln(stdout, "  -i  print the index number of each file")
			fmt.Fprintln(stdout, "  -l  use a long listing format")
			fmt.Fprintln(stdout, "  -n  like -l, but list numeric user and group IDs")
			fmt.Fprintln(stdout, "  -Q  enclose entry names in double quotes")
			fmt.Fprintln(stdout, "  -R  list subdirectories recursively")
			fmt.Fprintln(stdout, "  -r  reverse order while sorting")
			fmt.Fprintln(stdout, "  -S  sort by file size, largest first")
			fmt.Fprintln(stdout, "  -t  sort by time, newest first")
			fmt.Fprintln(stdout, "  -u  with -lt, sort by access time")
			fmt.Fprintln(stdout, "  -c  with -lt, sort by change time")
			return 0
		case len(a) > 1 && a[0] == '-' && a[1] != '-':
			// Combined short flags: -la, -alt, -lRr.
			for _, r := range a[1:] {
				switch r {
				case 'a':
					o.all = true
				case 'A':
					o.almost = true
				case '1':
					o.one = true
				case 'd':
					o.directory = true
				case 'F':
					o.classify = true
				case 'h':
					o.human = true
				case 'i':
					o.inode = true
				case 'l':
					o.long = true
				case 'n':
					o.numeric, o.long = true, true
				case 'Q':
					o.quote = true
				case 'R':
					o.recurse = true
				case 'r':
					o.reverse = true
				case 'S':
					o.bySize = true
				case 't':
					o.byTime = true
				case 'u':
					o.byAccess, o.byTime = true, true
				case 'c':
					o.byChange, o.byTime = true, true
				default:
					// GNU says "invalid option -- 'Z'" and exits 2. The old
					// u-root command printed usage and then listed the
					// directory with exit 0, which reads as success.
					fmt.Fprintf(stderr, "ls: invalid option -- '%c'\n", r)
					fmt.Fprintln(stderr, "Try 'ls --help' for more information.")
					return 2
				}
			}
		default:
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}

	l := lister{o: o, w: stdout, errw: stderr, now: time.Now()}
	code := 0
	// Header rules follow GNU: names only when more than one operand was given
	// or -R walks into directories.
	headers := len(paths) > 1 || o.recurse
	for _, p := range paths {
		if err := l.show(p, headers, ""); err != nil {
			fmt.Fprintf(stderr, "ls: cannot access '%s': %v\n", p, reason(err))
			code = 2
		}
	}
	return code
}

type lister struct {
	o    options
	w    io.Writer
	errw io.Writer
	now  time.Time
}

// entry is one line's worth of a file: everything the formatters need.
type entry struct {
	name  string
	path  string
	info  os.FileInfo
	mode  os.FileMode
	size  int64
	mtime time.Time
}

func (l lister) show(path string, headers bool, display string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !fi.IsDir() || l.o.directory {
		entries := []entry{l.entry(path, fi)}
		l.write(entries, false)
		return nil
	}

	byName, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	var entries []entry
	if l.o.all {
		// . and .. are not in the directory, so they are looked up.
		for _, name := range []string{".", ".."} {
			if e, err := l.lookup(filepath.Join(path, name), name); err == nil {
				entries = append(entries, e)
			}
		}
	}
	for _, d := range byName {
		name := d.Name()
		if !l.o.all && !l.o.almost && strings.HasPrefix(name, ".") {
			continue
		}
		e, err := l.lookup(filepath.Join(path, name), name)
		if err != nil {
			// A file that vanished between ReadDir and Lstat is GNU's
			// "cannot access"; skipping it silently would hide it.
			fmt.Fprintf(l.errw, "ls: cannot access '%s': %v\n", filepath.Join(path, name), reason(err))
			continue
		}
		entries = append(entries, e)
	}
	l.sort(entries)

	shown := display
	if shown == "" {
		shown = path
	}
	if headers {
		fmt.Fprintf(l.w, "%s:\n", shown)
	}

	l.write(entries, true)

	if l.o.recurse {
		for _, e := range entries {
			if e.info.IsDir() && !isDotName(e.name) {
				if headers {
					fmt.Fprintln(l.w)
				}
				if err := l.show(e.path, true, subPath(path, e.name)); err != nil {
					fmt.Fprintf(l.errw, "ls: cannot open directory '%s': %v\n", e.path, reason(err))
				}
			}
		}
	}
	return nil
}

func (l lister) lookup(path, name string) (entry, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return entry{}, err
	}
	return l.entry(path, fi).withName(name), nil
}

func (l lister) entry(path string, fi os.FileInfo) entry {
	return entry{
		name:  filepath.Base(path),
		path:  path,
		info:  fi,
		mode:  winmode.Apply(path, fi).Mode(),
		size:  fi.Size(),
		mtime: fi.ModTime(),
	}
}

func (e entry) withName(name string) entry {
	e.name = name
	return e
}

func (l lister) sort(entries []entry) {
	// GNU sorts by name by default, byte by byte, and -t/-S override that.
	less := func(i, j int) bool { return entries[i].name < entries[j].name }
	switch {
	case l.o.bySize:
		less = func(i, j int) bool {
			if entries[i].size != entries[j].size {
				return entries[i].size > entries[j].size
			}
			return entries[i].name < entries[j].name
		}
	case l.o.byTime:
		less = func(i, j int) bool {
			// Whole seconds, like GNU: two files written in the same second are
			// a tie, and then the name decides.
			ti, tj := l.sortTime(entries[i]).Truncate(time.Second), l.sortTime(entries[j]).Truncate(time.Second)
			if !ti.Equal(tj) {
				return ti.After(tj)
			}
			return entries[i].name < entries[j].name
		}
	}
	sort.SliceStable(entries, less)
	if l.o.reverse {
		for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
			entries[i], entries[j] = entries[j], entries[i]
		}
	}
}

// write prints the entries. In the long form a directory listing starts with the
// total in 1K blocks, which GNU takes from the allocated blocks.
func (l lister) write(entries []entry, isDir bool) {
	if l.o.long {
		if isDir && len(entries) > 0 {
			// GNU sums each file's allocated 512-byte blocks, converts them to
			// 1K per file, and prints the sum; -h makes it "total 16K".
			var kb int64
			for _, e := range entries {
				kb += (e.blocks()*512 + 1023) / 1024
			}
			if l.o.human {
				fmt.Fprintf(l.w, "total %s\n", humanSize(kb*1024))
			} else {
				fmt.Fprintf(l.w, "total %d\n", kb)
			}
		}
		w := l.widthsOf(entries)
		for _, e := range entries {
			fmt.Fprintln(l.w, l.longText(e, w))
		}
		return
	}
	for _, e := range entries {
		if l.o.inode {
			fmt.Fprintf(l.w, "%d ", e.inode())
		}
		fmt.Fprintln(l.w, l.displayName(e))
	}
}

func isDotName(name string) bool { return name == "." || name == ".." }

// subPath keeps the operand's spelling: GNU prints "./sub:" when the operand was
// ".", and "dir/sub:" when it was "dir".
func subPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "/" + name
}

// reason turns Go's error into the tail GNU prints.
func reason(err error) string {
	if pe, ok := err.(*os.PathError); ok {
		return pe.Err.Error()
	}
	return err.Error()
}
