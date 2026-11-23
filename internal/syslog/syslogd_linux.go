//go:build !windows

package syslog

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

// FileWriter appends to a file and rotates it when it grows past Max: the old
// file becomes NAME.0, one backup, like BusyBox' syslogd with -s. Rotation here
// and not in a separate logrotate, because the writer knows the size and a second
// program that opens the same file is a race.
type FileWriter struct {
	Path string
	Max  int64
	mu   sync.Mutex
	f    *os.File
	size int64
}

// OpenFileWriter opens or creates the file and learns its current size, so a
// restart does not wait for the next -s to rotate.
func OpenFileWriter(path string, max int64) (*FileWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &FileWriter{Path: path, Max: max, f: f, size: st.Size()}, nil
}

// Write appends one line and rotates before it when the limit would be passed.
func (w *FileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.Max > 0 && w.size+int64(len(p)) > w.Max && w.size > 0 {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *FileWriter) rotate() error {
	if err := w.f.Close(); err != nil {
		return err
	}
	backup := w.Path + ".0"
	_ = os.Remove(backup)
	if err := os.Rename(w.Path, backup); err != nil {
		return err
	}
	f, err := os.OpenFile(w.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	w.f = f
	w.size = 0
	return nil
}

// Close closes the file.
func (w *FileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.f.Close()
}

// daemon is syslogd: a unix datagram socket and a file.
type daemon struct {
	Socket string
	Out    *FileWriter
	Stderr io.Writer
}

// RunSyslogd is the command.
//
//	syslogd [-O FILE] [-s BYTES] [-S SOCKET] [-n]
func RunSyslogd(name string, args []string, stdout, stderr io.Writer) int {
	path := "/var/log/messages"
	socket := "/dev/log"
	max := int64(200 * 1024) // BusyBox -s default
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			fmt.Fprintf(stdout, "usage: %s [-O FILE] [-s BYTES] [-S SOCKET]\n", name)
			fmt.Fprintln(stdout, "  -O FILE    log file (default /var/log/messages)")
			fmt.Fprintln(stdout, "  -s BYTES   rotate to FILE.0 when the file would pass this size (0 = never, default 204800)")
			fmt.Fprintf(stdout, "  -S SOCKET  socket to listen on (default /dev/log)\n")
			fmt.Fprintln(stdout, "  stores each line as it arrived (RFC 3164); read it with logread")
			return 0
		case a == "-O":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "%s: -O needs a file\n", name)
				return 2
			}
			path = args[i]
		case strings.HasPrefix(a, "-O"):
			path = strings.TrimPrefix(a, "-O")
		case a == "-s":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "%s: -s needs bytes\n", name)
				return 2
			}
			if _, err := fmt.Sscanf(args[i], "%d", &max); err != nil {
				fmt.Fprintf(stderr, "%s: -s %q is not a number\n", name, args[i])
				return 2
			}
		case strings.HasPrefix(a, "-s"):
			if _, err := fmt.Sscanf(strings.TrimPrefix(a, "-s"), "%d", &max); err != nil {
				fmt.Fprintf(stderr, "%s: -s %q is not a number\n", name, a[2:])
				return 2
			}
		case a == "-S":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "%s: -S needs a path\n", name)
				return 2
			}
			socket = args[i]
		case a == "-n" || a == "-f":
			// foreground is the only mode, accepted for BusyBox's sake
		default:
			fmt.Fprintf(stderr, "%s: unknown option %q\n", name, a)
			return 2
		}
	}

	out, err := OpenFileWriter(path, max)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	defer out.Close()

	// A leftover socket from an unclean stop makes Listen fail with
	// "address already in use"; remove it, but never a real file.
	if st, err := os.Lstat(socket); err == nil {
		if st.Mode()&os.ModeSocket == 0 {
			fmt.Fprintf(stderr, "%s: %s exists and is not a socket, refusing\n", name, socket)
			return 1
		}
		_ = os.Remove(socket)
	}
	addr, err := net.ResolveUnixAddr("unixgram", socket)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	conn, err := net.ListenUnixgram("unixgram", addr)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	defer func() {
		conn.Close()
		_ = os.Remove(socket)
	}()
	// /dev/log is world writable, as every logger expects.
	_ = os.Chmod(socket, 0o666)

	d := &daemon{Socket: socket, Out: out, Stderr: stderr}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)
	defer signal.Stop(stop)
	go func() {
		<-stop
		conn.Close()
	}()

	fmt.Fprintf(stdout, "syslogd: %s -> %s\n", socket, path)
	if err := d.serve(conn); err != nil && !errors.Is(err, net.ErrClosed) {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	return 0
}

// serve reads datagrams until the socket is closed.
func (d *daemon) serve(conn *net.UnixConn) error {
	buf := make([]byte, 64*1024)
	for {
		n, _, err := conn.ReadFromUnix(buf)
		if err != nil {
			return err
		}
		line, ok := normalizeLine(string(buf[:n]))
		if !ok {
			continue
		}
		if _, err := d.Out.Write([]byte(line)); err != nil {
			fmt.Fprintf(d.Stderr, "syslogd: writing: %v\n", err)
		}
	}
}

// RunLogger is the command: one message onto /dev/log.
//
//	logger [-t TAG] [-p PRI] [-s] [--] [message...]
//
// Without arguments the message is read from stdin, one line per message, which
// is what a pipe needs (`tail -f x | logger -t x`).
func RunLogger(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	tag := filepath.Base(os.Args[0])
	prio := 13 // user.notice
	alsoStderr := false
	socket := "/dev/log"
	var words []string
	readStdin := true
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			fmt.Fprintf(stdout, "usage: %s [-t TAG] [-p PRI] [-s] [-S SOCKET] [message...]\n", name)
			fmt.Fprintln(stdout, "  -t TAG     tag in the line (default: the program name)")
			fmt.Fprintln(stdout, "  -p PRI     priority as facility.level, level or number (default user.notice)")
			fmt.Fprintln(stdout, "  -s         also write the line to stderr")
			fmt.Fprintf(stdout, "  -S SOCKET  socket (default /dev/log)\n")
			fmt.Fprintln(stdout, "  without a message the lines of stdin are sent, one each")
			return 0
		case a == "-t":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "%s: -t needs a tag\n", name)
				return 2
			}
			tag = args[i]
		case strings.HasPrefix(a, "-t"):
			tag = strings.TrimPrefix(a, "-t")
		case a == "-p":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "%s: -p needs a priority\n", name)
				return 2
			}
			p, err := ParsePriority(args[i])
			if err != nil {
				fmt.Fprintf(stderr, "%s: %v\n", name, err)
				return 2
			}
			prio = p
		case strings.HasPrefix(a, "-p"):
			p, err := ParsePriority(a[2:])
			if err != nil {
				fmt.Fprintf(stderr, "%s: %v\n", name, err)
				return 2
			}
			prio = p
		case a == "-s":
			alsoStderr = true
		case a == "-S":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "%s: -S needs a path\n", name)
				return 2
			}
			socket = args[i]
		case a == "--":
			words = append(words, args[i+1:]...)
			readStdin = false
			i = len(args)
		case strings.HasPrefix(a, "-") && len(a) > 1:
			fmt.Fprintf(stderr, "%s: unknown option %q\n", name, a)
			return 2
		default:
			words = append(words, a)
			readStdin = false
		}
	}

	conn, err := net.Dial("unixgram", socket)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	defer conn.Close()

	send := func(msg string) int {
		line := Format(prio, tag, os.Getpid(), msg, time.Now())
		if _, err := conn.Write([]byte(line)); err != nil {
			fmt.Fprintf(stderr, "%s: %v\n", name, err)
			return 1
		}
		if alsoStderr {
			fmt.Fprintln(stderr, line)
		}
		return 0
	}

	if !readStdin {
		return send(strings.Join(words, " "))
	}
	r := stdin
	if r == nil {
		r = os.Stdin
	}
	code := 0
	scanner := newLineScanner(r)
	for scanner.Scan() {
		if c := send(scanner.Text()); c != 0 {
			code = c
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	return code
}
