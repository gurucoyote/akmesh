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
// Local changes, all of them for the way agents call xargs:
//   - package xargs and Run(args, stdin, stdout, stderr) instead of main();
//   - no flag package: -I{} and -d, attach their value to the switch, which the
//     flag package reads as an unknown name;
//   - new: -I REPL (one invocation per line, REPL replaced everywhere), -r /
//     --no-run-if-empty, -d DELIM, -P NUM (run that many at once);
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
const tty = "/dev/tty"

var errTooSmall = errors.New("too small")
var errHelp = errors.New("help requested")

const usage = "usage: xargs [-0|-d DELIM] [-n NUM] [-I REPL] [-P NUM] [-r] [-t] [COMMAND [ARGS]...]"

type params struct {
	maxArgs  int
	trace    bool
	prompt   bool
	null     bool
	delim    string
	replace  string
	noRun    bool
	parallel int
}

type cmd struct {
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	tty    string
	params
}

func command(stdin io.Reader, stdout, stderr io.Writer, p params) *cmd {
	return &cmd{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		tty:    tty,
		params: p,
	}
}

// parse reads the options the way they are written: -I{} and -d, carry their
// value in the same token.
func parse(args []string) (rest []string, p params, err error) {
	p.maxArgs = defaultMaxArgs
	p.parallel = 1
	for i := 0; i < len(args); i++ {
		a := args[i]
		value := func(name string) (string, error) {
			if len(a) > len(name) {
				return a[len(name):], nil
			}
			if i+1 >= len(args) {
				return "", fmt.Errorf("%s needs a value", name)
			}
			i++
			return args[i], nil
		}
		switch {
		case a == "--":
			return append(rest, args[i+1:]...), p, nil
		case strings.HasPrefix(a, "-I"):
			v, err := value("-I")
			if err != nil {
				return nil, p, err
			}
			p.replace = v
		case a == "-n" || strings.HasPrefix(a, "-n"):
			v, err := value("-n")
			if err != nil {
				return nil, p, err
			}
			n, err := strconv.Atoi(v)
			if err != nil {
				return nil, p, fmt.Errorf("-n: invalid number %q", v)
			}
			p.maxArgs = n
		case strings.HasPrefix(a, "-d"):
			v, err := value("-d")
			if err != nil {
				return nil, p, err
			}
			p.delim = v
		case strings.HasPrefix(a, "-P"):
			v, err := value("-P")
			if err != nil {
				return nil, p, err
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				return nil, p, fmt.Errorf("-P: invalid number of processes %q", v)
			}
			p.parallel = n
		case a == "-r" || a == "--no-run-if-empty":
			p.noRun = true
		case a == "-t" || a == "--verbose":
			p.trace = true
		case a == "-p" || a == "--interactive":
			p.prompt = true
			p.trace = true
		case a == "-0" || a == "--null":
			p.null = true
		case a == "--help":
			return nil, p, errHelp
		case strings.HasPrefix(a, "-") && len(a) > 1:
			return nil, p, fmt.Errorf("unknown option %q", a)
		default:
			rest = append(rest, a)
		}
	}
	return rest, p, nil
}

// readItems splits the input into the strings every command gets appended.
func (c *cmd) readItems() ([]string, error) {
	data, err := io.ReadAll(c.stdin)
	if err != nil {
		return nil, err
	}
	switch {
	case c.replace != "":
		// -I works line by line: the whitespace inside a line belongs to the
		// item, which is what makes filenames with spaces work.
		text := strings.TrimRight(string(data), "\n")
		if text == "" {
			return nil, nil
		}
		return strings.Split(text, "\n"), nil
	case c.null:
		parts := strings.Split(string(data), "\x00")
		if len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
		return parts, nil
	case c.delim != "":
		parts := strings.Split(string(data), c.delim)
		if len(parts) > 0 && parts[len(parts)-1] == "" {
			parts = parts[:len(parts)-1]
		}
		return parts, nil
	}
	var items []string
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		items = append(items, strings.Fields(sc.Text())...)
	}
	return items, sc.Err()
}

// batches groups the items into the argument lists to run.
func (c *cmd) batches(args []string, items []string) [][]string {
	if c.replace != "" {
		out := make([][]string, 0, len(items))
		for _, item := range items {
			argv := make([]string, len(args))
			for i, a := range args {
				argv[i] = strings.ReplaceAll(a, c.replace, item)
			}
			out = append(out, argv)
		}
		return out
	}
	var out [][]string
	for i := 0; i < len(items); i += c.maxArgs {
		m := min(i+c.maxArgs, len(items))
		argv := append([]string{}, args...)
		argv = append(argv, items[i:m]...)
		out = append(out, argv)
	}
	return out
}

func (c *cmd) run(args ...string) int {
	if c.maxArgs <= 0 {
		fmt.Fprintf(c.stderr, "xargs: -n %d: %v\n", c.maxArgs, errTooSmall)
		return 1
	}
	if len(args) == 0 {
		args = []string{"echo"}
	}
	items, err := c.readItems()
	if err != nil {
		fmt.Fprintf(c.stderr, "xargs: %v\n", err)
		return 1
	}
	if len(items) == 0 && c.noRun {
		return 0
	}
	if len(items) == 0 && c.replace != "" {
		return 0
	}
	batches := c.batches(args, items)
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
		if c.prompt {
			fmt.Fprintf(c.stderr, "%s ...?", strings.Join(argv, " "))
			if !promptScanner.Scan() {
				break
			}
			answer := promptScanner.Text()
			if !strings.HasPrefix(answer, "y") && !strings.HasPrefix(answer, "Y") {
				continue
			}
		} else if c.trace {
			fmt.Fprintf(c.stderr, "%s\n", strings.Join(argv, " "))
		}
		sem <- struct{}{}
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

// Run implements xargs for the acmesh dispatcher.
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
