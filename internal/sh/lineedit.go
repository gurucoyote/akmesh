package sh

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// errLineCancelled is Ctrl-C in the editor: the line is dropped, the session
// keeps running. It is not the interrupter's business, because raw mode turns
// the terminal's own signal off and the byte arrives as 0x03.
var errLineCancelled = errors.New("line cancelled")

// historyLimit keeps the file small; ash and bash default to similar numbers.
const historyLimit = 1000

// historyFile names the file, honouring AKMESH_HISTFILE like ash honours
// HISTFILE. Without a home there is no file, and history stays in memory.
func historyFile() string {
	if p := os.Getenv("AKMESH_HISTFILE"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".akmesh_history")
}

// loadHistory reads the stored lines, newest last, capped at the limit.
func loadHistory() []string {
	name := historyFile()
	if name == "" {
		return nil
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > historyLimit {
		lines = lines[len(lines)-historyLimit:]
	}
	return lines
}

// appendHistory adds one line to the file: no empty lines, no immediate
// repetitions, and the file never grows past the limit.
func appendHistory(line string) {
	line = strings.TrimRight(line, "\n")
	if strings.TrimSpace(line) == "" {
		return
	}
	name := historyFile()
	if name == "" {
		return
	}
	var lines []string
	if data, err := os.ReadFile(name); err == nil {
		lines = strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	}
	if n := len(lines); n > 0 && lines[n-1] == line {
		return
	}
	lines = append(lines, line)
	if len(lines) > historyLimit {
		lines = lines[len(lines)-historyLimit:]
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		return
	}
	_ = os.WriteFile(name, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

// editor reads one line in raw mode: cursor keys, history and the usual
// control keys, so an agent or a person at a terminal can correct a typo
// instead of starting over (acmesh-mc52).
type editor struct {
	fd      int
	file    *os.File
	out     io.Writer
	history []string
	pos     int
	// override is the continuation prompt (PS2) for the next line, set while a
	// statement is still incomplete.
	override string
	// dir and getenv answer with the shell's own values, not this process':
	// tab completion has to look where the session is, and along its PATH
	// (measured 25.11.2026: cd moves the runner, not the process).
	dir    func() string
	getenv func(string) string
}

// setPrompt overrides the prompt for the next readLine, once.
func (e *editor) setPrompt(p string) { e.override = p }

func newEditor(f *os.File, out io.Writer) *editor {
	return &editor{fd: int(f.Fd()), file: f, out: out, history: loadHistory(),
		dir: func() string { d, _ := os.Getwd(); return d }, getenv: os.Getenv}
}

// readKey reads one byte; escape sequences are read on demand by the caller.
func (e *editor) readByte() (byte, error) {
	var b [1]byte
	n, err := e.file.Read(b[:])
	if n == 1 {
		return b[0], nil
	}
	if err == nil {
		err = io.EOF
	}
	return 0, err
}

func (e *editor) render(prompt string, buf []rune, pos int) {
	// \r, clear to end of line, the whole line, then back to the cursor. Raw mode
	// does not translate \n, so every line break is written as \r\n.
	s := "\r\x1b[K" + prompt + string(buf)
	if back := len(buf) - pos; back > 0 {
		s += fmt.Sprintf("\x1b[%dD", back)
	}
	fmt.Fprint(e.out, s)
}

// readLine edits one line. The returned string carries no newline.
func (e *editor) readLine(prompt string) (string, error) {
	if e.override != "" {
		prompt, e.override = e.override, ""
	}
	state, err := term.MakeRaw(e.fd)
	if err != nil {
		return "", err
	}
	defer func() { _ = term.Restore(e.fd, state) }()

	var buf []rune
	pos := 0
	e.pos = len(e.history)
	e.render(prompt, buf, pos)
	for {
		b, err := e.readByte()
		if err != nil {
			fmt.Fprint(e.out, "\r\n")
			return string(buf), err
		}
		switch b {
		case 0x03: // Ctrl-C
			fmt.Fprint(e.out, "\r\n")
			return "", errLineCancelled
		case 0x04: // Ctrl-D
			if len(buf) == 0 {
				fmt.Fprint(e.out, "\r\n")
				return "", io.EOF
			}
			if pos < len(buf) {
				buf = append(buf[:pos], buf[pos+1:]...)
			}
		case '\r', '\n':
			fmt.Fprint(e.out, "\r\n")
			line := string(buf)
			if strings.TrimSpace(line) != "" {
				e.history = append(e.history, line)
				appendHistory(line)
			}
			return line, nil
		case 0x7f, 0x08: // backspace
			if pos > 0 {
				atEnd := pos == len(buf)
				buf = append(buf[:pos-1], buf[pos:]...)
				pos--
				if atEnd {
					fmt.Fprint(e.out, "\b \b")
					continue
				}
			}
		case 0x01: // Ctrl-A
			pos = 0
		case 0x05: // Ctrl-E
			pos = len(buf)
		case 0x02: // Ctrl-B
			if pos > 0 {
				pos--
			}
		case 0x06: // Ctrl-F
			if pos < len(buf) {
				pos++
			}
		case 0x0b: // Ctrl-K
			buf = buf[:pos]
		case 0x15: // Ctrl-U
			buf = buf[pos:]
			pos = 0
		case 0x17: // Ctrl-W
			start := pos
			for start > 0 && buf[start-1] == ' ' {
				start--
			}
			for start > 0 && buf[start-1] != ' ' {
				start--
			}
			buf = append(buf[:start], buf[pos:]...)
			pos = start
		case 0x0c: // Ctrl-L
			fmt.Fprint(e.out, "\x1b[H\x1b[2J")
		case 0x09: // Tab: complete a command name or a path
			buf, pos = e.completeLine(buf, pos, prompt)
		case 0x1b: // escape sequence
			second, err := e.readByte()
			if err != nil {
				return string(buf), err
			}
			if second != '[' && second != 'O' {
				continue // a lone escape: ignore
			}
			third, err := e.readByte()
			if err != nil {
				return string(buf), err
			}
			switch third {
			case 'A': // up
				if e.pos > 0 {
					e.pos--
					buf = []rune(e.history[e.pos])
					pos = len(buf)
				}
			case 'B': // down
				if e.pos < len(e.history)-1 {
					e.pos++
					buf = []rune(e.history[e.pos])
				} else {
					e.pos = len(e.history)
					buf = nil
				}
				pos = len(buf)
			case 'C': // right
				if pos < len(buf) {
					pos++
				}
			case 'D': // left
				if pos > 0 {
					pos--
				}
			case 'H': // home
				pos = 0
			case 'F': // end
				pos = len(buf)
			case '1', '3', '4', '7', '8': // 1~ home, 3~ delete, 4~/8~ end, 7~ home
				if _, err := e.readByte(); err != nil { // the trailing '~'
					return string(buf), err
				}
				switch third {
				case '1', '7':
					pos = 0
				case '3':
					if pos < len(buf) {
						buf = append(buf[:pos], buf[pos+1:]...)
					}
				case '4', '8':
					pos = len(buf)
				}
			}
		default:
			if b >= 0x20 && b != 0x7f {
				// Printable ASCII, or the first byte of a UTF-8 sequence.
				r := []rune{rune(b)}
				if b >= 0xc0 {
					if decoded, n := e.readUTF8(b); n > 0 {
						r = decoded
					}
				}
				atEnd := pos == len(buf)
				buf = append(buf[:pos], append(r, buf[pos:]...)...)
				pos += len(r)
				if atEnd {
					// Typing at the end only adds text: echo it instead of
					// redrawing the whole line, so a screenreader does not
					// read the line again on every key (acmesh-mc52).
					fmt.Fprint(e.out, string(r))
					continue
				}
			}
		}
		e.render(prompt, buf, pos)
	}
}

// readUTF8 completes a multi-byte character whose first byte is already read.
func (e *editor) readUTF8(first byte) ([]rune, int) {
	n := 1
	switch {
	case first >= 0xf0:
		n = 4
	case first >= 0xe0:
		n = 3
	case first >= 0xc0:
		n = 2
	}
	b := []byte{first}
	for i := 1; i < n; i++ {
		c, err := e.readByte()
		if err != nil {
			break
		}
		b = append(b, c)
	}
	return []rune(string(b)), 1
}

// useEditor reports whether the interactive line editor may run: a real
// terminal, not a pipe, and not a dumb one.
func useEditor(stdin io.Reader) (*os.File, bool) {
	f, ok := stdin.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return nil, false
	}
	if os.Getenv("TERM") == "dumb" {
		return nil, false
	}
	return f, true
}
