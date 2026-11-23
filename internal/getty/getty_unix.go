//go:build !windows

// Package getty opens a terminal, prints /etc/issue, asks for the user name and
// hands the session to login. When the session ends it starts over, which is what
// init expects from a respawn service (acmesh-bu1).
//
// It calls internal/login in the same process instead of exec'ing /bin/login:
// same binary, same policy, no second password check, and no dependency on a
// login file being present in the image.
package getty

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"akmesh/internal/login"
)

// Paths and functions are variables so a test can point them somewhere else.
var (
	issueFile = "/etc/issue"
	hostname  = os.Hostname
	openTTY   = openTerminal
	runLogin  = defaultLogin
)

// Run is the command entry point.
//
//	getty [-f ISSUE] [-t TTY] [-l PROGRAM] [tty]
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	issuePath := issueFile
	loginProgram := "login"
	var ttyPath string
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "-h" || a == "--help":
			fmt.Fprintln(stdout, "usage: getty [-f ISSUE] [-t TTY] [-l PROGRAM] [tty]")
			fmt.Fprintln(stdout, "  -f ISSUE   file printed before the prompt (default /etc/issue)")
			fmt.Fprintln(stdout, "  -t TTY     terminal to open; without it stdin is used when it is a terminal")
			fmt.Fprintln(stdout, "  -l PROGRAM accepted for BusyBox's sake; the login is always ours")
			fmt.Fprintln(stdout, "  escapes in the issue: \\\\n hostname, \\\\l tty, \\\\t time, \\\\\\\\ backslash")
			return 0
		case a == "-f":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "getty: -f needs a file\n")
				return 2
			}
			issuePath = args[i]
		case a == "-t":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "getty: -t needs a terminal\n")
				return 2
			}
			ttyPath = args[i]
		case a == "-l":
			i++
			if i >= len(args) {
				fmt.Fprintf(stderr, "getty: -l needs a program\n")
				return 2
			}
			loginProgram = args[i]
			_ = loginProgram // accepted, see the note above
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(stderr, "getty: unknown option %q\n", a)
			return 2
		default:
			ttyPath = a
		}
	}

	session, err := openSession(ttyPath, stdin, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "getty: %v\n", err)
		return 1
	}
	defer session.Close()

	name := "tty"
	if ttyPath != "" {
		name = strings.TrimPrefix(ttyPath, "/dev/")
	}
	issue, err := os.ReadFile(issuePath)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(stderr, "getty: %v\n", err)
	}
	host, _ := hostname()

	// One session per run, like agetty and BusyBox: after a logout getty ends,
	// and init's respawn starts the next one. A getty that loops inside itself
	// would keep a broken terminal forever.
	if len(issue) > 0 {
		fmt.Fprint(session.out, expandIssue(string(issue), host, name, time.Now()))
	}
	var user string
	for {
		fmt.Fprintf(session.out, "%s login: ", host)
		line, err := readLine(session.in)
		if err != nil {
			// The terminal is gone before anyone logged in.
			return 0
		}
		user = strings.TrimSpace(line)
		if user == "" {
			continue
		}
		break
	}
	code := runLogin(user, session.in, session.out, session.err)
	fmt.Fprintf(session.out, "logout (exit %d)\n", code)
	return 0
}

// session is the terminal getty works on.
type session struct {
	in   io.Reader
	out  io.Writer
	err  io.Writer
	file *os.File
}

func (s *session) Close() {
	if s.file != nil {
		s.file.Close()
	}
}

// openSession opens the tty of the command line, or uses stdin when it is a
// terminal. A pipe on stdin is accepted too: that is how the container proof
// drives getty, and refusing it would make the program untestable.
func openSession(ttyPath string, stdin io.Reader, stdout, stderr io.Writer) (*session, error) {
	if ttyPath == "" {
		if f, ok := stdin.(*os.File); ok {
			return &session{in: f, out: f, err: f}, nil
		}
		return &session{in: stdin, out: stdout, err: stderr}, nil
	}
	f, err := openTTY(ttyPath)
	if err != nil {
		return nil, err
	}
	return &session{in: f, out: f, err: f, file: f}, nil
}

func openTerminal(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDWR, 0)
}

// readLine reads one line without a terminal library: getty only needs the name,
// and the password is login's business (which turns echo off itself).
func readLine(r io.Reader) (string, error) {
	var b strings.Builder
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			c := buf[0]
			if c == '\n' {
				return b.String(), nil
			}
			if c != '\r' {
				b.WriteByte(c)
			}
		}
		if err != nil {
			if b.Len() > 0 {
				return b.String(), nil
			}
			return "", err
		}
	}
}

// expandIssue fills the escapes of /etc/issue the way agetty documents them.
// Unknown escapes stay as they are, so a typo is visible instead of eaten.
func expandIssue(text, host, tty string, at time.Time) string {
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		if text[i] != '\\' || i+1 >= len(text) {
			b.WriteByte(text[i])
			continue
		}
		i++
		switch text[i] {
		case 'n':
			b.WriteString(host)
		case 'l':
			b.WriteString(tty)
		case 't':
			b.WriteString(at.Format("15:04:05"))
		case 'd':
			b.WriteString(at.Format("Jan 02 2006"))
		case '\\':
			b.WriteByte('\\')
		default:
			b.WriteByte('\\')
			b.WriteByte(text[i])
		}
	}
	return b.String()
}

// defaultLogin runs the session in this process.
func defaultLogin(name string, in io.Reader, out, err io.Writer) int {
	return login.Run([]string{name}, in, out, err)
}
