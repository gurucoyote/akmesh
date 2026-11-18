package corecmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// runEcho follows GNU echo, not u-root's: escapes are NOT interpreted unless
// -e is given. u-root's echo interprets them by default, which turns a literal
// backslash sequence in a script into a newline and silently changes output.
func runEcho(args []string, _ *os.File, stdout, _ *os.File) int {
	noNewline, escapes := false, false
	for len(args) > 0 && len(args[0]) > 1 && args[0][0] == '-' {
		rest := args[0][1:]
		ok := true
		for i := 0; i < len(rest); i++ {
			switch rest[i] {
			case 'n':
				noNewline = true
			case 'e':
				escapes = true
			case 'E':
				escapes = false
			default:
				ok = false
			}
		}
		if !ok {
			break
		}
		args = args[1:]
	}
	text := strings.Join(args, " ")
	if escapes {
		var stop bool
		text, stop = expandEscapes(text)
		if stop {
			noNewline = true
		}
	}
	if !noNewline {
		text += "\n"
	}
	if _, err := fmt.Fprint(stdout, text); err != nil {
		return 1
	}
	return 0
}

// expandEscapes implements the sequences GNU echo -e knows. \c stops all output
// and suppresses the trailing newline.
func expandEscapes(s string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'a':
			b.WriteByte(7)
		case 'b':
			b.WriteByte(8)
		case 'c':
			return b.String(), true
		case 'e', 'E':
			b.WriteByte(27)
		case 'f':
			b.WriteByte(12)
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'v':
			b.WriteByte(11)
		case '\\':
			b.WriteByte('\\')
		case '0':
			// \0NNN is an octal byte, one to three digits.
			j := i + 1
			for j < len(s) && j <= i+3 && s[j] >= '0' && s[j] <= '7' {
				j++
			}
			if v, err := strconv.ParseUint(s[i+1:j], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i = j - 1
			} else {
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
		case 'x':
			j := i + 1
			for j < len(s) && j <= i+2 && hexDigitByte(s[j]) {
				j++
			}
			if v, err := strconv.ParseUint(s[i+1:j], 16, 8); err == nil && j > i+1 {
				b.WriteByte(byte(v))
				i = j - 1
			} else {
				b.WriteByte('\\')
				b.WriteByte(s[i])
			}
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String(), false
}

// hexDigitByte is local because the sort package keeps its own copy.
func hexDigitByte(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// runPwd prints the working directory. -P resolves symlinks, -L keeps the
// logical path from the environment, which is also the default as in GNU.
func runPwd(args []string, _ *os.File, stdout, _ *os.File) int {
	physical := false
	for _, a := range args {
		switch a {
		case "-P":
			physical = true
		case "-L":
			physical = false
		default:
			fmt.Fprintf(os.Stderr, "pwd: unknown option %q\n", a)
			return 2
		}
	}
	dir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "pwd: %v\n", err)
		return 1
	}
	if physical {
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
	}
	fmt.Fprintln(stdout, dir)
	return 0
}

// runTrue and runFalse exist so a script's `if cmd; then` works the way an
// agent expects, without a shell builtin.
func runTrue(args []string, _ *os.File, _ *os.File, _ *os.File) int  { return 0 }
func runFalse(args []string, _ *os.File, _ *os.File, _ *os.File) int { return 1 }
