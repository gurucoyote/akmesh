package syslog

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// RunLogread is the command: print the log file, optionally following it.
//
//	logread [-f] [-n LINES] [-O FILE]
//
// -f is what makes it usable in a container: the file is read from the start and
// then followed, the way tail -f works, so `logread -f` can be a service in
// /etc/inittab.
func RunLogread(name string, args []string, stdout, stderr io.Writer) int {
	path := "/var/log/messages"
	follow := false
	tail := 0
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			fmt.Fprintf(stdout, "usage: %s [-f] [-n LINES] [-O FILE]\n", name)
			fmt.Fprintln(stdout, "  -f         follow the file, print lines as they arrive")
			fmt.Fprintln(stdout, "  -n LINES   only the last LINES lines")
			fmt.Fprintf(stdout, "  -O FILE    log file (default /var/log/messages)\n")
			fmt.Fprintln(stdout, "  lines are shown as they were stored, RFC 3164")
			return 0
		case a == "-f":
			follow = true
		case a == "-O":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "%s: -O needs a file\n", name)
				return 2
			}
			path = args[i]
		case strings.HasPrefix(a, "-O"):
			path = strings.TrimPrefix(a, "-O")
		case a == "-n":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "%s: -n needs a count\n", name)
				return 2
			}
			if _, err := fmt.Sscanf(args[i], "%d", &tail); err != nil {
				fmt.Fprintf(stderr, "%s: -n %q is not a number\n", name, args[i])
				return 2
			}
		case strings.HasPrefix(a, "-n"):
			if _, err := fmt.Sscanf(a[2:], "%d", &tail); err != nil {
				fmt.Fprintf(stderr, "%s: -n %q is not a number\n", name, a[2:])
				return 2
			}
		default:
			fmt.Fprintf(stderr, "%s: unknown option %q\n", name, a)
			return 2
		}
	}

	// What is already in the file is printed first, also with -f: that is what
	// BusyBox' logread -f does, and an operator who starts it wants the history
	// as well as the new lines.
	lines, err := readTail(path, tail)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	for _, l := range lines {
		fmt.Fprintln(stdout, l)
	}
	if !follow {
		return 0
	}
	return followFile(name, path, stdout, stderr)
}

// readTail reads the file and keeps the last n non-empty lines (n <= 0: all).
func readTail(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var lines []string
	scanner := newLineScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines, nil
}

// followFile prints new lines until the process is killed. It polls instead of
// using inotify: a container's log file is written by us and a 100 ms poll costs
// nothing, while inotify needs a second syscall family and misses rotation.
func followFile(name, path string, stdout, stderr io.Writer) int {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	buf := make([]byte, 0, 64*1024)
	chunk := make([]byte, 32*1024)
	for {
		n, err := f.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			for {
				i := bytes.IndexByte(buf, '\n')
				if i < 0 {
					break
				}
				fmt.Fprintln(stdout, string(buf[:i]))
				buf = buf[i+1:]
			}
		}
		if err == io.EOF {
			// Rotation: the file this handle points at shrinks or is replaced.
			if st, statErr := os.Stat(path); statErr == nil {
				if cur, posErr := f.Seek(0, io.SeekCurrent); posErr == nil && st.Size() < cur {
					_ = f.Close()
					newF, openErr := os.Open(path)
					if openErr == nil {
						f = newF
						continue
					}
				}
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", name, err)
			return 1
		}
	}
}
