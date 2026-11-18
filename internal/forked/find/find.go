// Copyright 2013-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Find finds files. It is similar to the Unix command.
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/find/find.go.
// The rest of this file is the upstream source and keeps its copyright.
//
// Local changes, all of them for the way agents call find:
//   - package find and Run(args, stdin, stdout, stderr) instead of main();
//   - the flag package is gone. GNU puts the start path first and the
//     predicates after it and allows -maxdepth between them, which flag
//     cannot read;
//   - the start path is optional and defaults to ".";
//   - new: -maxdepth N, -mindepth N, -path PATTERN, -not / !, -print0 and
//     -exec CMD... ; ;
//   - -l is gone with pkg/ls, its only user;
//   - a walk error or a failing -exec exits 1, an unknown predicate prints the
//     usage and exits 2 (upstream exited 0 for an unreadable directory);
//   - with the start path "." the printed paths keep the "./" prefix GNU
//     prints, because agents compare that text.
package find

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var errUsage = errors.New("usage: find [PATH...] [PREDICATE...]")

// action is what happens with a match: print it (the default), print it
// NUL-terminated, or run a command.
type action struct {
	exec   []string
	print0 bool
}

// options carries what the parser found.
type options struct {
	roots  []string
	sets   []Set
	action action
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: find [PATH...] [PREDICATE...]")
	fmt.Fprintln(w, "  predicates: -name PATTERN -path PATTERN -type f|d|l -mode OCTAL -regex RE")
	fmt.Fprintln(w, "              -maxdepth N -mindepth N -not / !")
	fmt.Fprintln(w, "  actions:    -print -print0 -exec CMD {} ;")
}

// fileTypes maps the GNU letters to the mode bits find compares against.
var fileTypes = map[string]os.FileMode{
	"f":         0,
	"file":      0,
	"d":         os.ModeDir,
	"directory": os.ModeDir,
	"s":         os.ModeSocket,
	"p":         os.ModeNamedPipe,
	"l":         os.ModeSymlink,
	"c":         os.ModeCharDevice | os.ModeDevice,
	"b":         os.ModeDevice,
}

// parse walks the arguments in order, because GNU lets the path come first and
// mixes predicates with the depth limits.
func parse(args []string) (*options, error) {
	o := &options{}
	maxDepth, minDepth := 0, 0
	negate := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "-d":
			// upstream's debug switch, kept so an old command line still works
			continue
		case "-maxdepth", "-mindepth":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("%s needs a number", a)
			}
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return nil, fmt.Errorf("%s: invalid number %q", a, args[i])
			}
			if a == "-maxdepth" {
				maxDepth = n
			} else {
				minDepth = n
			}
		case "-name", "-path", "-regex", "-type", "-mode", "-perm":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("%s needs a value", a)
			}
			i++
			value := args[i]
			var set Set
			switch a {
			case "-name":
				set = WithFilenameMatch(value)
			case "-path":
				set = WithPathMatch(value)
			case "-regex":
				compiled, err := WithCompiledRegexPathMatch(value)
				if err != nil {
					return nil, err
				}
				set = compiled
			case "-type":
				bits, ok := fileTypes[value]
				if !ok {
					return nil, fmt.Errorf("not a valid file type: %q", value)
				}
				set = WithModeMatch(bits, os.ModeType)
			case "-mode", "-perm":
				n, err := strconv.ParseUint(value, 8, 32)
				if err != nil {
					return nil, fmt.Errorf("%s: invalid mode %q", a, value)
				}
				set = WithModeMatch(os.FileMode(n), os.ModePerm)
			}
			if negate {
				set = WithInverted(set)
				negate = false
			}
			o.sets = append(o.sets, set)
		case "-not", "!":
			negate = true
		case "-print":
		case "-print0":
			o.action.print0 = true
		case "-exec":
			var argv []string
			closed := false
			for j := i + 1; j < len(args); j++ {
				if args[j] == ";" || args[j] == "+" {
					i = j
					closed = true
					break
				}
				argv = append(argv, args[j])
			}
			if !closed || len(argv) == 0 {
				return nil, errors.New("-exec needs a command and a terminating ';'")
			}
			o.action.exec = argv
		case "--":
			o.roots = append(o.roots, args[i+1:]...)
			i = len(args)
		default:
			if strings.HasPrefix(a, "-") && a != "-" {
				return nil, fmt.Errorf("unknown predicate %q", a)
			}
			if len(o.sets) > 0 || negate {
				return nil, fmt.Errorf("unexpected path %q after a predicate", a)
			}
			o.roots = append(o.roots, a)
		}
	}
	if negate {
		return nil, errors.New("-not needs a predicate after it")
	}
	if len(o.roots) == 0 {
		o.roots = []string{"."}
	}
	if maxDepth > 0 {
		o.sets = append(o.sets, WithMaxDepth(maxDepth))
	}
	if minDepth > 0 {
		o.sets = append(o.sets, WithMinDepth(minDepth))
	}
	return o, nil
}

// display is the path as GNU prints it: with the start path "." the "./" prefix
// stays, because agents compare that text.
func display(root, name string) string {
	if root != "." || name == "." || filepath.IsAbs(name) ||
		strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") {
		return name
	}
	return "./" + name
}

// runExec starts the -exec command once per match with {} replaced.
func runExec(argv []string, name string, stdin io.Reader, stdout, stderr io.Writer) error {
	replaced := make([]string, len(argv))
	for i, a := range argv {
		replaced[i] = strings.ReplaceAll(a, "{}", name)
	}
	cmd := exec.Command(replaced[0], replaced[1:]...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// Run implements find for the acmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	o, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "find: %v\n", err)
		usage(stderr)
		return 2
	}
	code := 0
	for _, root := range o.roots {
		if _, err := os.Lstat(root); err != nil {
			fmt.Fprintf(stderr, "find: %v\n", err)
			code = 1
			continue
		}
		sets := append([]Set{WithRoot(root)}, o.sets...)
		for f := range Find(context.Background(), sets...) {
			if f.Err != nil {
				fmt.Fprintf(stderr, "find: %v\n", f.Err)
				code = 1
				continue
			}
			name := display(root, f.Name)
			switch {
			case o.action.exec != nil:
				if err := runExec(o.action.exec, f.Name, stdin, stdout, stderr); err != nil {
					var exitErr *exec.ExitError
					if !errors.As(err, &exitErr) {
						fmt.Fprintf(stderr, "find: %v\n", err)
						code = 1
					}
				}
			case o.action.print0:
				fmt.Fprintf(stdout, "%s\x00", name)
			default:
				fmt.Fprintf(stdout, "%s\n", name)
			}
		}
	}
	return code
}
