//go:build windows

package syslog

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RunSyslogd has nothing to do on Windows: a /dev/log unix datagram socket does
// not exist, and this program leaves no traces in the host - the Windows event
// log is a host facility (AGENTS.md, "Windows: no traces in the host"). The
// honest answer is a refusal, not a fake socket.
func RunSyslogd(name string, args []string, stdout, stderr io.Writer) int {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Fprintf(stdout, "usage: %s [-O FILE] [-s BYTES] [-S SOCKET]\n", name)
			fmt.Fprintln(stdout, "on Windows there is no /dev/log; write the log with 'logger -O FILE' instead")
			return 0
		}
	}
	fmt.Fprintf(stderr, "%s: not supported on Windows (no /dev/log; use logger -O FILE)\n", name)
	return 1
}

// RunLogger on Windows writes the line into a local file, because there is no
// socket to send it to. Nothing is registered with the host.
func RunLogger(name string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	tag := filepath.Base(os.Args[0])
	prio := 13
	alsoStderr := false
	path := ""
	var words []string
	readStdin := true
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			fmt.Fprintf(stdout, "usage: %s [-t TAG] [-p PRI] [-s] -O FILE [-L] [message...]\n", name)
			fmt.Fprintln(stdout, "  -O FILE    log file to append to (required on Windows, -L is accepted for compatibility)")
			fmt.Fprintln(stdout, "  -t TAG     tag in the line (default: the program name)")
			fmt.Fprintln(stdout, "  -p PRI     facility.level, level or number (default user.notice)")
			fmt.Fprintln(stdout, "  -s         also write the line to stderr")
			return 0
		case a == "-O" || a == "-L":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "%s: %s needs a file\n", name, a)
				return 2
			}
			path = args[i]
		case a == "-s":
			alsoStderr = true
		case a == "-t":
			i++
			if i < len(args) {
				tag = args[i]
			}
		case a == "-p":
			i++
			if i < len(args) {
				p, err := ParsePriority(args[i])
				if err != nil {
					fmt.Fprintf(stderr, "%s: %v\n", name, err)
					return 2
				}
				prio = p
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			fmt.Fprintf(stderr, "%s: unknown option %q\n", name, a)
			return 2
		default:
			words = append(words, a)
			readStdin = false
		}
	}
	if path == "" {
		fmt.Fprintf(stderr, "%s: no log file: pass -O FILE (no /dev/log on Windows)\n", name)
		return 2
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", name, err)
		return 1
	}
	defer f.Close()
	send := func(msg string) int {
		line := Format(prio, tag, os.Getpid(), msg, time.Now())
		if _, err := fmt.Fprintln(f, line); err != nil {
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
	return code
}
