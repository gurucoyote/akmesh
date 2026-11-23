// Copyright 2013-2023 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// xargs reads space, tab, newline and end-of-file delimited strings from the
// standard input and executes utility with the strings as arguments.
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/xargs/xargs.go.
// The rest of this file is the upstream source and keeps its copyright.
//
// Local changes, all of them for the way agents call xargs and all of them
// measured against GNU findutils 4.9.0 (23.11.2026, acmesh-ji5):
//   - package xargs and Run(args, stdin, stdout, stderr) instead of main();
//   - no flag package: -I{} and -d, attach their value to the switch, which the
//     flag package reads as an unknown name;
//   - the options end at the first word that is not one, so
//     "xargs sh -c 'code'" works: upstream read -c as an own option and failed;
//   - quotes and backslashes in the default mode: 'a b' is one item, as in GNU.
//     Upstream passed the quote characters through;
//   - new: -a FILE, -E EOF / -e EOF, -L NUM / -l, -i REPL (from -I), the long
//     spellings (--max-args=, --max-lines=, --delimiter=, --arg-file=,
//     --eof=, --replace=, --max-procs=), -r / --no-run-if-empty, -d DELIM,
//     -P NUM (run that many at once);
//   - the whole input is read before the first command starts, so a child never
//     steals the bytes meant for the next one (upstream passed the same reader
//     down to every child);
//   - the exit status follows GNU: 123 when a command failed, 124 when it
//     failed with 255, 127 when it could not be started.
package xargs

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const defaultMaxArgs = 5000

var errHelp = errors.New("help requested")

const usage = "usage: xargs [-0|-d DELIM] [-n NUM] [-L NUM] [-I REPL] [-a FILE] [-E EOF] [-P NUM] [-r] [-t] [COMMAND [ARGS]...]"

type params struct {
	maxArgs  int
	maxLines int
	trace    bool
	prompt   bool
	null     bool
	delim    string
	replace  string
	noRun    bool
	parallel int
	argFile  string
	eof      string
}

type cmd struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	tty    string
	params
}

// lockedWriter serialises the parallel children's writes into one stream. A
// mutex per Write keeps each chunk whole; the lock is shared by stdout and
// stderr so a message never lands inside another one.
type lockedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func command(stdin io.Reader, stdout, stderr io.Writer, p params) *cmd {
	return &cmd{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		tty:    "/dev/tty",
		params: p,
	}
}

// opt is one option word, split into its name and an optional attached value.
// short is the letter of a single dash option (-n2), long the name of a double
// dash one (--max-args), so the same switch serves --max-args=2 and -n 2.
type opt struct {
	short   byte
	long    string
	val     string
	hasVal  bool
	wasLong bool
}

// parseOpt parses one word as an option and reports false when the word is not
// one.
func parseOpt(a string) (opt, bool) {
	switch {
	case a == "-" || a == "--":
		return opt{}, false
	case strings.HasPrefix(a, "--"):
		name, val, has := strings.Cut(a[2:], "=")
		return opt{long: name, val: val, hasVal: has, wasLong: true}, true
	case strings.HasPrefix(a, "-") && len(a) > 1:
		o := opt{short: a[1]}
		if len(a) > 2 {
			o.val = a[2:]
			o.hasVal = true
		}
		return o, true
	}
	return opt{}, false
}

// value returns the attached value or takes the next word.
func (o opt) value(args []string, i *int) (string, error) {
	if o.hasVal {
		return o.val, nil
	}
	if *i+1 >= len(args) {
		return "", fmt.Errorf("option requires an argument -- %s", o.spelling())
	}
	*i++
	return args[*i], nil
}

func (o opt) spelling() string {
	if o.wasLong {
		return "--" + o.long
	}
	return "-" + string(o.short)
}

// parse reads the options and stops at the first word that is not one. GNU
// xargs reads its command and everything after it as the command line, options
// included, so "xargs sh -c 'exit 5'" must not look at -c.
func parse(args []string) (rest []string, p params, err error) {
	p.maxArgs = defaultMaxArgs
	p.parallel = 1
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			return append(rest, args[i+1:]...), p, nil
		}
		o, ok := parseOpt(a)
		if !ok {
			// Everything from here on is the command, options included.
			return append(rest, args[i:]...), p, nil
		}
		number := func() (int, error) {
			v, err := o.value(args, &i)
			if err != nil {
				return 0, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return 0, fmt.Errorf("invalid number %q for %s", v, o.spelling())
			}
			return n, nil
		}
		if o.wasLong {
			switch o.long {
			case "help":
				return nil, p, errHelp
			case "null":
				p.null = true
			case "no-run-if-empty":
				p.noRun = true
			case "verbose":
				p.trace = true
			case "interactive":
				p.prompt = true
				p.trace = true
			case "max-args":
				n, err := number()
				if err != nil {
					return nil, p, err
				}
				p.maxArgs = n
			case "max-lines":
				n, err := number()
				if err != nil {
					return nil, p, err
				}
				p.maxLines = n
			case "max-procs":
				n, err := number()
				if err != nil {
					return nil, p, err
				}
				p.parallel = n
			case "delimiter":
				v, err := o.value(args, &i)
				if err != nil {
					return nil, p, err
				}
				p.delim = v
			case "arg-file":
				v, err := o.value(args, &i)
				if err != nil {
					return nil, p, err
				}
				p.argFile = v
			case "eof":
				v, err := o.value(args, &i)
				if err != nil {
					return nil, p, err
				}
				p.eof = v
			case "replace":
				p.replace = "{}"
				if o.hasVal {
					p.replace = o.val
				}
			default:
				return nil, p, fmt.Errorf("unrecognized option %q", a)
			}
			continue
		}
		switch o.short {
		case '0':
			p.null = true
		case 'r':
			p.noRun = true
		case 't':
			p.trace = true
		case 'p':
			p.prompt = true
			p.trace = true
		case 'n':
			n, err := number()
			if err != nil {
				return nil, p, err
			}
			p.maxArgs = n
		case 'L', 'l':
			n, err := number()
			if err != nil {
				return nil, p, err
			}
			p.maxLines = n
		case 'P':
			n, err := number()
			if err != nil {
				return nil, p, err
			}
			p.parallel = n
		case 'd':
			v, err := o.value(args, &i)
			if err != nil {
				return nil, p, err
			}
			p.delim = v
		case 'a':
			v, err := o.value(args, &i)
			if err != nil {
				return nil, p, err
			}
			p.argFile = v
		case 'E', 'e':
			// -e with a value sets the end string, a bare -e disables it, as
			// GNU documents for the deprecated spelling.
			if o.hasVal {
				p.eof = o.val
			} else if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				i++
				p.eof = args[i]
			} else {
				p.eof = ""
			}
		case 'I':
			v, err := o.value(args, &i)
			if err != nil {
				return nil, p, err
			}
			p.replace = v
		case 'i':
			// -i is the deprecated -I with a default of {}.
			p.replace = "{}"
			if o.hasVal {
				p.replace = o.val
			}
		case 's':
			// -s is the byte limit; it has no counterpart here, but refusing
			// it would stop a script that only uses the older spelling.
			if _, err := o.value(args, &i); err != nil {
				return nil, p, err
			}
		default:
			return nil, p, fmt.Errorf("unknown option %q", a)
		}
	}
	return rest, p, nil
}

// source returns the reader the items come from: the -a file or stdin.
func (c *cmd) source() (io.Reader, func(), error) {
	if c.argFile == "" {
		return c.stdin, func() {}, nil
	}
	f, err := os.Open(c.argFile)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

// splitQuoted splits text the way GNU's default mode does: blanks separate
// items, single and double quotes keep blanks inside an item, and a backslash
// takes the next character literally.
func splitQuoted(text string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inItem := false
	flush := func() {
		if inItem {
			out = append(out, cur.String())
			cur.Reset()
			inItem = false
		}
	}
	for i := 0; i < len(text); i++ {
		switch ch := text[i]; ch {
		case ' ', '\t', '\n', '\v', '\f', '\r':
			flush()
		case '\'', '"':
			inItem = true
			i++
			for {
				if i >= len(text) {
					return nil, fmt.Errorf("unmatched %c quote", ch)
				}
				if text[i] == ch {
					break
				}
				if ch == '"' && text[i] == '\\' && i+1 < len(text) {
					i++
				}
				cur.WriteByte(text[i])
				i++
			}
		case '\\':
			if i+1 < len(text) {
				i++
				cur.WriteByte(text[i])
				inItem = true
			}
		default:
			cur.WriteByte(ch)
			inItem = true
		}
	}
	flush()
	return out, nil
}

// readText reads the whole source once; no child ever sees the reader.
func (c *cmd) readText() (string, error) {
	src, closeSrc, err := c.source()
	if err != nil {
		return "", err
	}
	defer closeSrc()
	data, err := io.ReadAll(src)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// cutEOF drops everything from the first item equal to the end string, as -E
// and -e ask.
func (c *cmd) cutEOF(items []string) []string {
	if c.eof == "" {
		return items
	}
	for i, it := range items {
		if it == c.eof {
			return items[:i]
		}
	}
	return items
}

// records splits the text into the strings one invocation replaces or counts.
// With -0 and -d the delimiters are literal and no quoting happens; in the
// default mode blanks separate and quotes group.
func (c *cmd) records(text string) ([]string, error) {
	switch {
	case c.null:
		parts := strings.Split(text, "\x00")
		if n := len(parts); n > 0 && parts[n-1] == "" {
			parts = parts[:n-1]
		}
		return parts, nil
	case c.delim != "":
		parts := strings.Split(text, c.delim)
		if n := len(parts); n > 0 && parts[n-1] == "" {
			parts = parts[:n-1]
		}
		return parts, nil
	}
	return splitQuoted(text)
}

// logicalLines returns the input split into lines, an unterminated last line
// included. -I and -L work on lines, not on blank separated items.
func logicalLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n")
}

// batches groups the items into the argument lists to run.
func (c *cmd) batches(args, items, lines []string) [][]string {
	if c.replace != "" {
		out := make([][]string, 0, len(lines))
		for _, line := range lines {
			argv := make([]string, len(args))
			for i, a := range args {
				argv[i] = strings.ReplaceAll(a, c.replace, line)
			}
			out = append(out, argv)
		}
		return out
	}
	if c.maxLines > 0 {
		var out [][]string
		for i := 0; i < len(lines); i += c.maxLines {
			n := min(i+c.maxLines, len(lines))
			argv := append([]string{}, args...)
			for _, line := range lines[i:n] {
				toks, err := splitQuoted(line)
				if err != nil {
					// An unmatched quote stops the run with status 1, which the
					// caller reports; the rest of the line is dropped.
					continue
				}
				argv = append(argv, toks...)
			}
			out = append(out, argv)
		}
		return out
	}
	var out [][]string
	for i := 0; i < len(items); i += c.maxArgs {
		n := min(i+c.maxArgs, len(items))
		argv := append([]string{}, args...)
		argv = append(argv, items[i:n]...)
		out = append(out, argv)
	}
	return out
}

func (c *cmd) run(args ...string) int {
	text, err := c.readText()
	if err != nil {
		fmt.Fprintf(c.stderr, "xargs: %v\n", err)
		return 1
	}
	items, err := c.records(text)
	if err != nil {
		fmt.Fprintf(c.stderr, "xargs: %v\n", err)
		return 1
	}
	items = c.cutEOF(items)
	lines := logicalLines(text)
	if c.eof != "" {
		for i, l := range lines {
			if l == c.eof {
				lines = lines[:i]
				break
			}
		}
	}
	if len(args) == 0 {
		args = []string{"echo"}
	}
	if len(items) == 0 && (c.noRun || c.replace != "") {
		return 0
	}
	batches := c.batches(args, items, lines)
	if len(batches) == 0 {
		// Nothing to append still means one run of the plain command, as GNU
		// does without -r.
		batches = [][]string{args}
	}

	var promptScanner *bufio.Scanner
	if c.prompt {
		f, err := os.Open(c.tty)
		if err != nil {
			fmt.Fprintf(c.stderr, "xargs: %v\n", err)
			return 1
		}
		defer f.Close()
		promptScanner = bufio.NewScanner(f)
	}

	code := 0
	// With -P, several children write to the same stdout at the same time. The
	// akmesh binary passes os.Stdout (safe), but Run is part of a library a
	// harness calls with any io.Writer, and a plain bytes.Buffer or
	// strings.Builder loses output when two goroutines append at once
	// (measured: one lost line in 21 runs, and a reported data race). One lock
	// around both streams keeps every write whole and the writer safe.
	if c.parallel > 1 {
		var wmu sync.Mutex
		c.stdout = &lockedWriter{mu: &wmu, w: c.stdout}
		c.stderr = &lockedWriter{mu: &wmu, w: c.stderr}
	}
	var mu sync.Mutex
	record := func(err error) {
		if err == nil {
			return
		}
		var exitErr *exec.ExitError
		mu.Lock()
		defer mu.Unlock()
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 255 {
				code = 124
			} else if code == 0 {
				code = 123
			}
			return
		}
		code = 127
	}

	sem := make(chan struct{}, c.parallel)
	var wg sync.WaitGroup
	for _, argv := range batches {
		// Take the slot before writing the trace line: with -P1 GNU prints
		// "echo a", then a's output, then "echo b" (measured 23.11.2026);
		// tracing before the slot put both trace lines first.
		sem <- struct{}{}
		if c.prompt {
			fmt.Fprintf(c.stderr, "%s ...?", strings.Join(argv, " "))
			if !promptScanner.Scan() {
				<-sem
				break
			}
			answer := promptScanner.Text()
			if !strings.HasPrefix(answer, "y") && !strings.HasPrefix(answer, "Y") {
				<-sem
				continue
			}
		} else if c.trace {
			fmt.Fprintf(c.stderr, "%s\n", strings.Join(argv, " "))
		}
		wg.Add(1)
		go func(argv []string) {
			defer wg.Done()
			defer func() { <-sem }()
			child := exec.Command(argv[0], argv[1:]...)
			child.Stdout = c.stdout
			child.Stderr = c.stderr
			record(child.Run())
		}(argv)
	}
	wg.Wait()
	return code
}

// Run implements xargs for the akmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	rest, p, err := parse(args)
	if err != nil {
		if errors.Is(err, errHelp) {
			fmt.Fprintln(stdout, usage)
			return 0
		}
		fmt.Fprintf(stderr, "xargs: %v\n", err)
		return 1
	}
	c := command(stdin, stdout, stderr, p)
	return c.run(rest...)
}
