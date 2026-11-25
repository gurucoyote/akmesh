// Copyright 2013-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// find finds files the way GNU find does it: start paths, then an expression
// built from predicates, operators and actions.
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76 (cmds/core/find,
// pkg/find); the licence above stays. The walk and the expression are new,
// because upstream has a flat list of matchers, no -o, no parentheses and no
// negation tree.
//
// Measured against GNU find 4.9 and kept:
//   - the start path is optional and defaults to ".";
//   - entries come in directory order, not sorted (acmesh-kgi);
//   - a path under the root "." keeps its "./" prefix, also for -exec {};
//   - -maxdepth 0 reports the root alone and does not descend;
//   - precedence ! > -a > -o, "(" ")" group, an action suppresses -print;
//   - a broken expression exits 2 with the usage line, an unreadable directory
//     or a failing -exec exits 1.
package find

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var errUsage = errors.New("usage: find [PATH...] [EXPRESSION]")

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: find [PATH...] [EXPRESSION]")
	fmt.Fprintln(w, "  tests:      -name PAT -iname PAT -path PAT -ipath PAT -regex RE")
	fmt.Fprintln(w, "              -type f|d|l|s|p|c|b -size N[cwbkMG] -empty")
	fmt.Fprintln(w, "              -newer FILE -mtime N -mmin N -user NAME -group NAME")
	fmt.Fprintln(w, "              -perm [-]MODE -mode OCTAL")
	fmt.Fprintln(w, "  options:    -maxdepth N -mindepth N -depth -xdev -follow")
	fmt.Fprintln(w, "  actions:    -print -print0 -printf FORMAT -exec CMD {} ; -exec CMD {} +")
	fmt.Fprintln(w, "              -delete -quit")
	fmt.Fprintln(w, "  operators:  ! / -not, -a / -and (implied), -o / -or, ( )")
}

// entry is what the expression is evaluated against.
type entry struct {
	path  string // the path as printed, with the "./" prefix under the root "."
	name  string // base name
	root  string
	depth int
	info  os.FileInfo
}

// runner carries the state of one find invocation.
type runner struct {
	out    io.Writer
	errOut io.Writer
	stdin  io.Reader
	opts   options
	now    time.Time
	quit   bool
	code   int
	// -exec CMD {} + collects its matches and runs once at the end.
	plusCmd []string
	plus    []string
	dev     uint64
	hasDev  bool
}

// printf writes one entry formatted with -printf.
func (r *runner) printf(format string, e *entry) {
	fmt.Fprint(r.out, printfFormat(unescapeFormat(format), e.info, e.path))
}

func (r *runner) print(e *entry, nul bool) {
	if nul {
		fmt.Fprintf(r.out, "%s\x00", e.path)
		return
	}
	fmt.Fprintln(r.out, e.path)
}

// isEmpty is -empty: a zero-length file or a directory without entries.
func (r *runner) isEmpty(e *entry) bool {
	if e.info.IsDir() {
		f, err := os.Open(e.path)
		if err != nil {
			return false
		}
		defer f.Close()
		names, err := f.Readdirnames(1)
		return err != nil || len(names) == 0
	}
	return e.info.Size() == 0
}

func (r *runner) delete(e *entry) {
	if err := os.Remove(e.path); err != nil {
		fmt.Fprintf(r.errOut, "find: %v\n", err)
		r.code = 1
	}
}

// exec runs a command once per match, with {} replaced by the path.
func (r *runner) exec(argv []string, e *entry) {
	replaced := make([]string, len(argv))
	for i, a := range argv {
		replaced[i] = strings.ReplaceAll(a, "{}", e.path)
	}
	cmd := exec.Command(replaced[0], replaced[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = r.stdin, r.out, r.errOut
	if err := cmd.Run(); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			fmt.Fprintf(r.errOut, "find: %v\n", err)
			r.code = 1
		}
	}
}

// execPlus collects the matches for one run at the end, which is what
// "-exec CMD {} +" means.
func (r *runner) execPlus(argv []string, e *entry) {
	if r.plusCmd == nil {
		r.plusCmd = argv
	}
	r.plus = append(r.plus, e.path)
}

func (r *runner) runPlus() {
	if r.plusCmd == nil || len(r.plus) == 0 {
		return
	}
	var args []string
	for _, a := range r.plusCmd {
		// The placeholder carries the paths, so it is dropped here and the
		// matches are appended after the fixed arguments.
		if a == "{}" {
			continue
		}
		args = append(args, a)
	}
	args = append(args, r.plus...)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = r.stdin, r.out, r.errOut
	if err := cmd.Run(); err != nil {
		if _, ok := errors.AsType[*exec.ExitError](err); !ok {
			fmt.Fprintf(r.errOut, "find: %v\n", err)
			r.code = 1
		}
	}
}

// walk evaluates the expression over one path and, for a directory, over its
// contents. The order comes from Readdirnames, which does not sort, and that is
// the order GNU prints.
func (r *runner) walk(path, name, root string, depth int, expr node, info os.FileInfo) {
	e := &entry{path: path, name: name, root: root, depth: depth, info: info}
	if depth >= r.opts.minDepth && !r.opts.depthAlt {
		expr.eval(r, e)
		if r.quit {
			return
		}
	}
	if info.IsDir() && (r.opts.maxDepth < 0 || depth < r.opts.maxDepth) {
		f, err := os.Open(path)
		if err != nil {
			fmt.Fprintf(r.errOut, "find: %v\n", err)
			r.code = 1
			r.finish(e, expr)
			return
		}
		names, err := f.Readdirnames(-1)
		f.Close()
		if err != nil {
			fmt.Fprintf(r.errOut, "find: %v\n", err)
			r.code = 1
			r.finish(e, expr)
			return
		}
		for _, child := range names {
			if r.quit {
				break
			}
			childPath := filepath.Join(path, child)
			childInfo, cerr := os.Lstat(childPath)
			if cerr != nil {
				fmt.Fprintf(r.errOut, "find: %v\n", cerr)
				r.code = 1
				continue
			}
			if r.opts.xdev {
				if dev, ok := devOf(childInfo); ok && r.hasDev && dev != r.dev {
					continue
				}
			}
			if r.opts.follow && childInfo.Mode()&os.ModeSymlink != 0 {
				if followed, ferr := os.Stat(childPath); ferr == nil {
					childInfo = followed
				}
			}
			r.walk(display(root, childPath), child, root, depth+1, expr, childInfo)
		}
	}
	r.finish(e, expr)
}

// finish evaluates a directory in -depth mode, after its contents.
func (r *runner) finish(e *entry, expr node) {
	if !r.opts.depthAlt || r.quit {
		return
	}
	if e.depth >= r.opts.minDepth {
		expr.eval(r, e)
	}
}

// display keeps the "./" prefix of the start path ".", because agents compare
// that text with GNU's output.
func display(root, path string) string {
	if root != "." {
		return path
	}
	if path == "." || filepath.IsAbs(path) || strings.HasPrefix(path, "./") {
		return path
	}
	return "./" + path
}

// Run implements find for the akmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	p, expr, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "find: %v\n", err)
		usage(stderr)
		return 2
	}
	r := &runner{out: stdout, errOut: stderr, stdin: stdin, opts: p.opts, now: time.Now()}
	r.opts.maxDepth = p.opts.maxDepth
	// -delete works on a directory only after its contents, so GNU turns
	// -depth on for it.
	if usesDelete(expr) {
		r.opts.depthAlt = true
	}
	for _, root := range p.roots {
		info, lerr := os.Lstat(root)
		if lerr != nil && r.opts.follow {
			info, lerr = os.Stat(root)
		}
		if lerr != nil {
			fmt.Fprintf(stderr, "find: %v\n", lerr)
			r.code = 1
			continue
		}
		if dev, ok := devOf(info); ok {
			r.dev, r.hasDev = dev, true
		}
		r.walk(display(root, root), root, root, 0, expr, info)
		if r.quit {
			break
		}
	}
	r.runPlus()
	return r.code
}

// usesDelete reports whether the expression carries -delete.
func usesDelete(n node) bool {
	switch v := n.(type) {
	case *andNode:
		return usesDelete(v.left) || usesDelete(v.right)
	case *orNode:
		return usesDelete(v.left) || usesDelete(v.right)
	case *notNode:
		return usesDelete(v.inner)
	case *actionNode:
		return v.deletes
	}
	return false
}
