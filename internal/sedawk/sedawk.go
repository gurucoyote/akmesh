package sedawk

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
	"github.com/rwtodd/Go.Sed/sed"
)

// runSed implements sed through the Go.Sed engine: -e, -f, -n, -i and -E.
//
// The engine is forked (scripts/apply-go-sed-fork.sh): a basic regular
// expression is translated to Go's syntax, so \(...\)/\1 and & work, the
// leftmost longest match wins, and "." matches a newline. Without -E the
// pattern is read the way GNU sed reads it; with -E the operators are written
// unescaped and nothing is translated. What stays open: back references inside
// the search pattern (\(a\)\1) need an engine with backtracking, and the error
// says so.
func runSed(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var programs []string
	quiet := false
	extended := false
	inPlace := false
	zeroTerminated := false
	var files []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-e", "--expression":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "sed: -e needs an argument")
				return 2
			}
			i++
			programs = append(programs, args[i])
		case "-f", "--file":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, "sed: -f needs an argument")
				return 2
			}
			i++
			data, err := os.ReadFile(args[i])
			if err != nil {
				fmt.Fprintf(stderr, "sed: %v\n", err)
				return 1
			}
			programs = append(programs, string(data))
		case "-E", "-r", "--regexp-extended":
			extended = true
		case "-n", "--quiet", "--silent":
			quiet = true
		case "-i", "--in-place":
			inPlace = true
		case "-z", "--zero-terminated", "--null-data":
			// GNU -z: records end at NUL instead of a line break, and the
			// output is separated by NUL too (acmesh-d9e).
			zeroTerminated = true
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" && !strings.HasPrefix(args[i], "--") && len(args[i]) > 2 {
				// Glued short flags, as an agent writes them: -zn, -nE, -Ez.
				bad := false
				for _, letter := range args[i][1:] {
					switch letter {
					case 'n':
						quiet = true
					case 'E', 'r':
						extended = true
					case 'z':
						zeroTerminated = true
					case 'i':
						inPlace = true
					default:
						bad = true
					}
				}
				if bad {
					fmt.Fprintf(stderr, "sed: unknown option %q\n", args[i])
					return 2
				}
				continue
			}
			if strings.HasPrefix(args[i], "-") && args[i] != "-" {
				fmt.Fprintf(stderr, "sed: unknown option %q\n", args[i])
				return 2
			}
			files = append(files, args[i])
		}
	}
	if len(programs) == 0 {
		if len(files) == 0 {
			fmt.Fprintln(stderr, "sed: no program")
			return 2
		}
		programs = append(programs, files[0])
		files = files[1:]
	}
	program := strings.Join(programs, "\n")
	if zeroTerminated {
		// A "\n" in the script means a line break inside the record, which the
		// stream carries as the two bytes 0x01 0x30; write it that way so a
		// pattern can match it and a replacement can produce it (acmesh-0l4).
		program = strings.ReplaceAll(program, `\n`, "\x01"+"0")
	}
	var eng *sed.Engine
	var err error
	switch {
	case quiet && extended:
		eng, err = sed.NewQuietExtended(strings.NewReader(program))
	case quiet:
		eng, err = sed.NewQuiet(strings.NewReader(program))
	case extended:
		eng, err = sed.NewExtended(strings.NewReader(program))
	default:
		eng, err = sed.New(strings.NewReader(program))
	}
	if err != nil {
		fmt.Fprintf(stderr, "sed: %v\n", err)
		return 1
	}
	// The engine reads records by line break, so -z translates the stream on
	// the way in and back on the way out: NUL becomes a line break (the record
	// separator the engine knows) and a literal line break inside a record
	// becomes 0x01 0x30; the byte 0x01 itself is stuffed as 0x01 0x31, which
	// keeps the mapping reversible for every input. Limitation, stated in
	// docs/measurements.md: a script that matches a line break inside a NUL
	// record sees the escape instead.
	var in io.Reader = stdin
	var out io.Writer = stdout
	var zstate termState
	if zeroTerminated {
		zstate = termState{}
		in = nulToLines(stdin, &zstate)
		out = linesToNul(stdout, &zstate)
	}
	if len(files) == 0 {
		if _, err := io.Copy(out, eng.Wrap(in)); err != nil {
			fmt.Fprintf(stderr, "sed: %v\n", err)
			return 1
		}
		if nw, ok := out.(*nulWriter); ok {
			if err := nw.finish(); err != nil {
				fmt.Fprintf(stderr, "sed: %v\n", err)
				return 1
			}
		}
		return 0
	}
	code := 0
	for _, name := range files {
		if name == "-" {
			io.Copy(out, eng.Wrap(in))
			continue
		}
		f, err := os.Open(name)
		if err != nil {
			fmt.Fprintf(stderr, "sed: %v\n", err)
			code = 1
			continue
		}
		var fin io.Reader = f
		if zeroTerminated {
			fin = nulToLines(f, &zstate)
		}
		if !inPlace {
			if _, err := io.Copy(out, eng.Wrap(fin)); err != nil {
				fmt.Fprintf(stderr, "sed: %v\n", err)
				code = 1
			}
			f.Close()
			continue
		}
		var sb strings.Builder
		if _, err := io.Copy(&sb, eng.Wrap(fin)); err != nil {
			fmt.Fprintf(stderr, "sed: %v\n", err)
			f.Close()
			code = 1
			continue
		}
		f.Close()
		info, _ := os.Stat(name)
		mode := os.FileMode(0o644)
		if info != nil {
			mode = info.Mode().Perm()
		}
		if err := os.WriteFile(name, []byte(sb.String()), mode); err != nil {
			fmt.Fprintf(stderr, "sed: %v\n", err)
			code = 1
		}
	}
	if nw, ok := out.(*nulWriter); ok {
		if err := nw.finish(); err != nil {
			fmt.Fprintf(stderr, "sed: %v\n", err)
			return 1
		}
	}
	return code
}

// termState remembers whether the input ended with a NUL. GNU keeps that
// property: a record without a terminator stays without one (measured
// 24.11.2026 against sed 4.9).
type termState struct {
	endedWithTerm bool
}

// nulToLines turns a NUL-separated stream into the line-separated one the sed
// engine wants, escaping the bytes that would be ambiguous.
func nulToLines(r io.Reader, st *termState) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		br := bufio.NewReaderSize(r, 64*1024)
		buf := make([]byte, 0, 64*1024)
		for {
			chunk, err := br.ReadBytes(0)
			body := []byte(strings.TrimSuffix(string(chunk), "\x00"))
			for _, b := range body {
				switch b {
				case '\n':
					buf = append(buf, 0x01, 0x30)
				case 0x01:
					buf = append(buf, 0x01, 0x31)
				default:
					buf = append(buf, b)
				}
			}
			if len(chunk) > 0 && chunk[len(chunk)-1] == 0 {
				buf = append(buf, '\n')
				st.endedWithTerm = true
			} else if len(chunk) > 0 {
				st.endedWithTerm = false
			}
			if _, werr := pw.Write(buf); werr != nil {
				return
			}
			buf = buf[:0]
			if err != nil {
				pw.Close()
				return
			}
		}
	}()
	return pr
}

// linesToNul is the way back: a line break is the record separator again, and
// the escapes become the bytes they stood for.
func linesToNul(w io.Writer, st *termState) io.Writer {
	return &nulWriter{w: w, st: st}
}

type nulWriter struct {
	w    io.Writer
	st   *termState
	held bool // a 0x01 waiting for its second byte
	// pendingNul holds the last NUL back: it is written only when more output
	// follows or when the input ended with a terminator itself.
	pendingNul bool
}

func (n *nulWriter) Write(p []byte) (int, error) {
	out := make([]byte, 0, len(p))
	for _, b := range p {
		if n.held {
			n.held = false
			switch b {
			case 0x30:
				out = append(out, '\n')
			case 0x31:
				out = append(out, 0x01)
			default:
				out = append(out, 0x01, b)
			}
			continue
		}
		switch b {
		case 0x01:
			n.held = true
		case '\n':
			out = append(out, 0)
		default:
			out = append(out, b)
		}
	}
	if len(out) > 0 {
		if n.pendingNul {
			out = append([]byte{0}, out...)
			n.pendingNul = false
		}
		if out[len(out)-1] == 0 {
			out = out[:len(out)-1]
			n.pendingNul = true
		}
		if _, err := n.w.Write(out); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// finish writes the held-back terminator when the input had one.
func (n *nulWriter) finish() error {
	if !n.pendingNul {
		return nil
	}
	n.pendingNul = false
	if n.st != nil && !n.st.endedWithTerm {
		return nil
	}
	_, err := n.w.Write([]byte{0})
	return err
}

// awkParseError turns goawk's parse message into the one mawk prints, where
// they differ: mawk says "function xor never defined", goawk "parse error at
// 1:11: undefined function \"xor\"" (measured 24.11.2026, acmesh-3xs). The
// rest of goawk's messages stay as they are.
func awkParseError(err error) string {
	msg := err.Error()
	const marker = "undefined function "
	if _, after, ok := strings.Cut(msg, marker); ok {
		name := strings.Trim(after, `\"`)
		if j := strings.IndexAny(name, " \t"); j >= 0 {
			name = name[:j]
		}
		if name != "" {
			return "function " + name + " never defined"
		}
	}
	return msg
}

// runAwk implements awk through goawk: -F, -v, -f, program and files.
func runAwk(args []string, stdin *os.File, stdout, stderr *os.File) int {
	var progFiles, vars []string
	fieldSep := ""
	var positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		attached := ""
		if len(a) > 2 && (a[:2] == "-F" || a[:2] == "-v" || a[:2] == "-f") {
			a, attached = a[:2], a[2:]
		}
		switch a {
		case "-F":
			if attached == "" {
				if i+1 >= len(args) {
					fmt.Fprintln(stderr, "awk: -F needs an argument")
					return 2
				}
				i++
				attached = args[i]
			}
			fieldSep = attached
		case "-v":
			if attached == "" {
				if i+1 >= len(args) {
					fmt.Fprintln(stderr, "awk: -v needs an argument")
					return 2
				}
				i++
				attached = args[i]
			}
			name, value, ok := strings.Cut(attached, "=")
			if !ok {
				fmt.Fprintf(stderr, "awk: -v wants name=value, got %q\n", attached)
				return 2
			}
			vars = append(vars, name, value)
		case "-f":
			if attached == "" {
				if i+1 >= len(args) {
					fmt.Fprintln(stderr, "awk: -f needs an argument")
					return 2
				}
				i++
				attached = args[i]
			}
			data, err := os.ReadFile(attached)
			if err != nil {
				fmt.Fprintf(stderr, "awk: %v\n", err)
				return 1
			}
			progFiles = append(progFiles, string(data))
		default:
			if strings.HasPrefix(args[i], "-") && args[i] != "-" && len(positional) == 0 {
				fmt.Fprintf(stderr, "awk: unknown option %q\n", args[i])
				return 2
			}
			positional = append(positional, args[i])
		}
	}
	source := strings.Join(progFiles, "\n")
	if source == "" {
		if len(positional) == 0 {
			fmt.Fprintln(stderr, "awk: no program")
			return 2
		}
		source = positional[0]
		positional = positional[1:]
	}
	prog, err := parser.ParseProgram([]byte(source), &parser.ParserConfig{})
	if err != nil {
		fmt.Fprintf(stderr, "awk: %s\n", awkParseError(err))
		return 2
	}
	if fieldSep != "" {
		vars = append(vars, "FS", fieldSep)
	}
	cfg := &interp.Config{
		Stdin:  stdin,
		Output: stdout,
		Error:  stderr,
		Argv0:  "awk",
		Args:   positional,
		Vars:   vars,
	}
	status, err := interp.ExecProgram(prog, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "awk: %v\n", err)
	}
	return status
}
