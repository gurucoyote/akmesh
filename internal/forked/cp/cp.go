// Copyright 2016-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// cp copies files.
//
// Synopsis:
//
//	cp [-rRfivwP] FROM... TO
//
// Options:
//
//	-w n: number of worker goroutines
//	-R: copy file hierarchies
//	-r: alias to -R recursive mode
//	-i: prompt about overwriting file
//	-f: force overwrite files
//	-v: verbose copy mode
//	-P: don't follow symlinks
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/cp/cp.go.
// The rest of this file is the upstream source and keeps its copyright.
// pkg/cp came along as cp_cp.go in this directory, with one change: an option
// to keep mode and time.
//
// Local changes, all of them for the way agents call cp:
//   - package cp and Run(args, stdin, stdout, stderr) instead of main();
//   - no flag package and no unixflag: an own parser, so -a and -t DIR work;
//   - new: -a (same as -R -p -P), -p (keep mode and time), -t DIR (target
//     directory, GNU's spelling), -n (never overwrite);
//   - -v prints "'src' -> 'dst'", the line GNU prints, so a recursive copy can
//     be read back;
//   - a failing source is reported and the remaining ones still copy, with
//     exit 1 at the end (upstream stopped at the first error).
package cp

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var errUsage = errors.New("usage: cp [-RrifvP] file[s] ... dest")

type flags struct {
	recursive        bool
	ask              bool
	force            bool
	verbose          bool
	noFollowSymlinks bool
	preserve         bool
	noClobber        bool
	targetDir        string
}

// promptOverwrite ask if the user wants overwrite file
func promptOverwrite(dst string, out io.Writer, in *bufio.Reader) (bool, error) {
	fmt.Fprintf(out, "cp: overwrite %q? ", dst)
	answer, err := in.ReadString('\n')
	if err != nil {
		return false, err
	}

	if strings.ToLower(answer)[0] != 'y' {
		return false, nil
	}

	return true, nil
}

func setupPreCallback(recursive, ask, force bool, writer io.Writer, reader bufio.Reader) func(string, string, os.FileInfo) error {
	return func(src, dst string, srcfi os.FileInfo) error {
		// check if src is dir
		if !recursive && srcfi.IsDir() {
			fmt.Fprintf(writer, "cp: -r not specified, omitting directory %s\n", src)
			return ErrSkip
		}

		dstfi, err := os.Stat(dst)
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(writer, "cp: %q: can't handle error %v\n", dst, err)
			return ErrSkip
		} else if err != nil {
			// dst does not exist.
			return nil
		}

		// dst does exist.

		if os.SameFile(srcfi, dstfi) {
			fmt.Fprintf(writer, "cp: %q and %q are the same file\n", src, dst)
			return ErrSkip
		}
		if ask && !force {
			overwrite, err := promptOverwrite(dst, writer, &reader)
			if err != nil {
				return err
			}
			if !overwrite {
				return ErrSkip
			}
		}
		return nil
	}
}

func setupPostCallback(verbose bool, w io.Writer) func(src, dst string) {
	return func(src, dst string) {
		if verbose {
			fmt.Fprintf(w, "'%s' -> '%s'\n", src, dst)
		}
	}
}

// parse reads the switches GNU cp has and agents write.
func parse(args []string) (rest []string, f flags, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return append(rest, args[i+1:]...), f, nil
		case a == "-t" || a == "--target-directory":
			if i+1 >= len(args) {
				return nil, f, errors.New("option -t needs a directory")
			}
			i++
			f.targetDir = args[i]
		case strings.HasPrefix(a, "--target-directory="):
			f.targetDir = strings.TrimPrefix(a, "--target-directory=")
		case strings.HasPrefix(a, "--"):
			switch a {
			case "--archive":
				f.recursive, f.preserve, f.noFollowSymlinks = true, true, true
			case "--preserve":
				f.preserve = true
			case "--recursive":
				f.recursive = true
			case "--interactive":
				f.ask = true
			case "--force":
				f.force = true
			case "--verbose":
				f.verbose = true
			case "--no-dereference":
				f.noFollowSymlinks = true
			case "--no-clobber":
				f.noClobber = true
			default:
				return nil, f, fmt.Errorf("unknown option %q", a)
			}
		case strings.HasPrefix(a, "-") && len(a) > 1:
			// -rRfivP can be bundled; -t swallows the rest of the token.
			rest := a[1:]
			for j := 0; j < len(rest); j++ {
				switch rest[j] {
				case 'R', 'r':
					f.recursive = true
				case 'a':
					f.recursive, f.preserve, f.noFollowSymlinks = true, true, true
				case 'p':
					f.preserve = true
				case 'i':
					f.ask = true
				case 'f':
					f.force = true
				case 'v':
					f.verbose = true
				case 'P':
					f.noFollowSymlinks = true
				case 'n':
					f.noClobber = true
				case 't':
					v := rest[j+1:]
					if v == "" {
						if i+1 >= len(args) {
							return nil, f, errors.New("option -t needs a directory")
						}
						i++
						v = args[i]
					}
					f.targetDir = v
					j = len(rest)
				default:
					return nil, f, fmt.Errorf("unknown option %q", "-"+string(rest[j]))
				}
			}
		default:
			rest = append(rest, a)
		}
	}
	return rest, f, nil
}

// preCallback is upstream's setupPreCallback plus the -n check.
func preCallback(f flags, writer io.Writer, reader *bufio.Reader) func(string, string, os.FileInfo) error {
	inner := setupPreCallback(f.recursive, f.ask, f.force, writer, *reader)
	return func(src, dst string, srcfi os.FileInfo) error {
		if f.noClobber {
			if _, err := os.Lstat(dst); err == nil {
				return ErrSkip
			}
		}
		return inner(src, dst, srcfi)
	}
}

// run evaluates the flags and args and makes decisions for copyfiles.
func run(args []string, stdin io.Reader, w, stderr io.Writer) int {
	rest, f, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "cp: %v\n", err)
		return 1
	}
	from, to := rest, ""
	if f.targetDir != "" {
		from, to = rest, f.targetDir
	} else {
		if len(rest) < 2 {
			fmt.Fprintln(stderr, errUsage)
			return 1
		}
		from, to = rest[:len(rest)-1], rest[len(rest)-1]
	}
	if len(from) == 0 {
		fmt.Fprintln(stderr, errUsage)
		return 1
	}

	todir := false
	if toStat, err := os.Stat(to); err == nil {
		todir = toStat.IsDir()
	}
	if len(from) > 1 && !todir && f.targetDir == "" {
		fmt.Fprintf(stderr, "cp: target '%s' is not a directory\n", to)
		return 1
	}

	opts := Options{
		NoFollowSymlinks: f.noFollowSymlinks,
		Preserve:         f.preserve,

		// cp the command makes sure that
		//
		// (1) the files it's copying aren't already the same,
		// (2) the user is asked about overwriting an existing file if
		//     one is already there.
		PreCallback: preCallback(f, w, bufio.NewReader(stdin)),

		PostCallback: setupPostCallback(f.verbose, w),
	}

	code := 0
	for _, file := range from {
		dst := to
		if todir || f.targetDir != "" {
			dst = filepath.Join(dst, filepath.Base(file))
		}
		var err error
		if f.recursive {
			err = opts.CopyTree(file, dst)
		} else {
			err = opts.Copy(file, dst)
		}
		if err != nil {
			// GNU names the source in its own wording; an agent reads that
			// line and not the Go error behind it.
			if errors.Is(err, os.ErrNotExist) {
				fmt.Fprintf(stderr, "cp: cannot stat '%s': No such file or directory\n", file)
			} else {
				fmt.Fprintf(stderr, "cp: %v\n", err)
			}
			code = 1
		}
	}
	return code
}

// Run implements cp for the acmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run(args, stdin, stdout, stderr)
}
