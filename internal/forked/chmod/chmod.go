// Copyright 2016-2020 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// chmod changes mode bits (e.g. permissions) of a file.
//
// Synopsis:
//
//	chmod MODE FILE...
//
// Description:
//
//	MODE is a three character octal value or a string like a=rwx
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/chmod/chmod.go.
// The rest of this file is the upstream source and keeps its copyright. The
// copy includes the patch scripts/patch-uroot.sh applies in the checkout
// (missing mode class defaults to "a", so +x works) - a fork must not lose it.
//
// Local changes:
//   - package chmod and Run(args, stdin, stdout, stderr) instead of main();
//   - no flag package: -R/-r, -v and -c are read by an own parser, because the
//     flag package only knew --recursive and --reference and refused the GNU
//     spellings agents type;
//   - -v and -c print the mode change in GNU's wording;
//   - a recursive walk reports the failing path and keeps going (exit 1), and
//     a missing operand prints the usage with exit 1, as GNU does.
package chmod

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	special = 99999
	usage   = "chmod: chmod [mode] filepath"
)

var errBadUsage = errors.New(usage)

func usageText(w io.Writer) {
	fmt.Fprintln(w, "usage: chmod [-R] [-v|-c] [--reference FILE] MODE FILE...")
}

// tryText is the hint GNU prints after an error. Agents read it, so it is the
// same wording, not our own.
func tryText(w io.Writer) {
	fmt.Fprintln(w, "Try 'chmod --help' for more information.")
}

type cmd struct {
	stderr    io.Writer
	stdout    io.Writer
	reference string
	recursive bool
	verbose   bool
	changed   bool
}

func command(stdin io.Reader, stdout, stderr io.Writer, recursive, verbose, changedOnly bool, reference string) *cmd {
	return &cmd{
		stderr:    stderr,
		stdout:    stdout,
		recursive: recursive,
		verbose:   verbose,
		changed:   changedOnly,
		reference: reference,
	}
}

func changeMode(path string, mode os.FileMode, octval uint64, mask uint64) error {
	// A special value for mask means the mode is fully described
	if mask == special {
		if err := os.Chmod(path, mode); err != nil {
			return err
		}
		return recordMode(path, mode)
	}

	var info os.FileInfo
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	mode = info.Mode() & os.FileMode(mask)
	mode = mode | modeBits(octval)

	if err := os.Chmod(path, mode); err != nil {
		return err
	}
	return recordMode(path, mode)
}

// modeBits translates an octal mode the way the shell and GNU chmod spell it
// (0755, 4755, 2755, 1777) into the bit layout Go uses, where setuid, setgid and
// the sticky bit live above the nine permission bits.
func modeBits(v uint64) os.FileMode {
	m := os.FileMode(v & 0o777)
	if v&0o4000 != 0 {
		m |= os.ModeSetuid
	}
	if v&0o2000 != 0 {
		m |= os.ModeSetgid
	}
	if v&0o1000 != 0 {
		m |= os.ModeSticky
	}
	return m
}

func calculateMode(modeString string) (mode os.FileMode, octval uint64, mask uint64, err error) {
	octval, err = strconv.ParseUint(modeString, 8, 32)
	if err == nil {
		// akmesh: 4755 is what "make su setuid" looks like, and GNU accepts it.
		// Refusing anything above 0777 made a setuid bit impossible to set.
		if octval > 0o7777 {
			return mode, octval, mask, fmt.Errorf("%w: invalid octal value %0o. Value should be less than or equal to 07777", strconv.ErrRange, octval)
		}
		// a fully described octal mode was supplied, signal that with a special value for mask
		mask = special
		mode = modeBits(octval)
		return
	}

	reMode := regexp.MustCompile("^([ugoa]*)([-+=])(.*)")
	m := reMode.FindStringSubmatch(modeString)
	// akmesh: GNU allows the class to be omitted; "+x" means "a+x".
	if len(m) >= 2 && m[1] == "" {
		m[1] = "a"
	}
	// Test for mode strings with invalid characters.
	// This can't be done in the first regexp: if the match for m[3] is restricted to [rwx]*,
	// `a=9` and `a=` would be indistinguishable: m[3] would be empty.
	// `a=` is a valid (but destructive) operation. Do not turn a typo into that.
	reMode = regexp.MustCompile("^[rwxst]*$")
	if len(m) < 3 || !reMode.MatchString(m[3]) {
		return mode, octval, mask, fmt.Errorf("invalid mode: '%s'", modeString)
	}

	// m[3] is [rwx]{0,3}
	var octvalDigit uint64
	if strings.Contains(m[3], "r") {
		octvalDigit += 4
	}
	if strings.Contains(m[3], "w") {
		octvalDigit += 2
	}
	if strings.Contains(m[3], "x") {
		octvalDigit++
	}

	// setuid, setgid and sticky: "u+s", "g+s", "o+t" (and "a+s", GNU's
	// "chmod +s"). Go keeps these in high mode bits, not in the octal digit, so
	// they are collected apart from the rwx bits and folded in by modeBits.
	if strings.Contains(m[3], "s") {
		class := uint64(0o4000) // u
		switch {
		case strings.Contains(m[1], "a"):
			class = 0o6000
		case strings.Contains(m[1], "g") && !strings.Contains(m[1], "u"):
			class = 0o2000
		}
		if m[2] == "-" {
			return mode, octval, mask, fmt.Errorf("removing setuid/setgid is not supported yet")
		}
		octval |= class
	}
	if strings.Contains(m[3], "t") {
		if m[2] == "-" {
			return mode, octval, mask, fmt.Errorf("removing the sticky bit is not supported yet")
		}
		octval |= 0o1000
	}

	// m[2] is [-+=]
	operator := m[2]

	// Use a mask so that we do not overwrite permissions for a user/group that was not specified
	mask = 0o777

	// For "-", invert octvalDigit before applying the mask
	if operator == "-" {
		octvalDigit = 7 - octvalDigit
	}

	// m[1] is [ugoa]+
	if strings.Contains(m[1], "o") || strings.Contains(m[1], "a") {
		octval += octvalDigit
		mask = mask & 0o770
	}
	if strings.Contains(m[1], "g") || strings.Contains(m[1], "a") {
		octval += octvalDigit << 3
		mask = mask & 0o707
	}
	if strings.Contains(m[1], "u") || strings.Contains(m[1], "a") {
		octval += octvalDigit << 6
		mask = mask & 0o077
	}

	// For "+" the mask is superfluous, reset it
	if operator == "+" {
		mask = 0o777
	}

	// The mode is fully described, signal that with a special value for mask
	if operator == "=" && strings.Contains(m[1], "a") {
		mask = special
		mode = modeBits(octval)
	}
	return mode, octval, mask, nil
}

func (c *cmd) run(args ...string) error {
	var mode os.FileMode
	if len(args) < 1 {
		return errBadUsage
	}

	if len(args) < 2 && c.reference == "" {
		return errBadUsage
	}

	var (
		octval, mask uint64
		fileList     []string
	)

	if c.reference != "" {
		fi, err := os.Stat(c.reference)
		if err != nil {
			return fmt.Errorf("bad reference file: %w", err)
		}
		mask = special
		mode = fi.Mode()
		fileList = args
	} else {
		var err error
		if mode, octval, mask, err = calculateMode(args[0]); err != nil {
			return err
		}
		fileList = args[1:]
	}

	var finalErr error

	for _, name := range fileList {
		if c.recursive {
			err := filepath.WalkDir(name, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				// WalkDir doesn't descend into symlinks but still reports
				// them, and os.Chmod follows them. Like GNU chmod, skip
				// any met during the traversal.
				if path != name && d.Type()&fs.ModeSymlink == fs.ModeSymlink {
					return nil
				}
				before, _ := os.Stat(path)
				err = changeMode(path, mode, octval, mask)
				if err == nil && before != nil && (c.verbose || c.changed) {
					after, stErr := os.Stat(path)
					if stErr == nil && (c.verbose || before.Mode().Perm() != after.Mode().Perm()) {
						describe(c.stdout, path, before.Mode(), after.Mode())
					}
				}
				return err
			})
			if err != nil {
				finalErr = err
				fmt.Fprintln(c.stderr, err)
			}
		} else {
			before, _ := os.Stat(name)
			err := changeMode(name, mode, octval, mask)
			if err == nil && before != nil && (c.verbose || c.changed) {
				after, stErr := os.Stat(name)
				if stErr == nil && (c.verbose || before.Mode().Perm() != after.Mode().Perm()) {
					describe(c.stdout, name, before.Mode(), after.Mode())
				}
			}
			if err != nil {
				finalErr = err
				fmt.Fprintln(c.stderr, err)
			}
		}
	}
	return finalErr
}

// describe prints a mode the way GNU does in -v and -c:
//
//	mode of 'f.txt' changed from 0755 (rwxr-xr-x) to 0655 (rw-r-xr-x)
func describe(stdout io.Writer, path string, before, after os.FileMode) {
	// GNU prints "rwxr-xr-x", without the type character Go prepends.
	perm := func(m os.FileMode) string {
		text := m.String()
		if len(text) == 10 {
			return text[1:]
		}
		return text
	}
	if before == after {
		fmt.Fprintf(stdout, "mode of '%s' retained as %04o (%s)\n", path, after.Perm(), perm(after))
		return
	}
	fmt.Fprintf(stdout, "mode of '%s' changed from %04o (%s) to %04o (%s)\n",
		path, before.Perm(), perm(before), after.Perm(), perm(after))
}

// parse reads the switches GNU chmod knows in the form agents write them.
func parse(args []string) (rest []string, recursive, verbose, changedOnly bool, reference string, err error) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--":
			return append(rest, args[i+1:]...), recursive, verbose, changedOnly, reference, nil
		case a == "--recursive":
			recursive = true
		case a == "--verbose":
			verbose = true
		case a == "--changes":
			changedOnly = true
		case a == "--silent" || a == "--quiet":
			// nothing is printed for a failing file either; kept quiet
		case a == "--reference":
			if i+1 >= len(args) {
				return nil, false, false, false, "", errors.New("--reference needs a file")
			}
			i++
			reference = args[i]
		case strings.HasPrefix(a, "--reference="):
			reference = strings.TrimPrefix(a, "--reference=")
		case strings.HasPrefix(a, "-") && len(a) > 1:
			for _, c := range a[1:] {
				switch c {
				case 'R', 'r':
					recursive = true
				case 'v':
					verbose = true
				case 'c':
					changedOnly = true
				case 'f':
					// -f silences the error messages; nothing else to do here
				default:
					return nil, false, false, false, "", fmt.Errorf("unknown option %q", "-"+string(c))
				}
			}
		default:
			rest = append(rest, a)
		}
	}
	return rest, recursive, verbose, changedOnly, reference, nil
}

// Run implements chmod for the akmesh dispatcher.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && (args[0] == "--help" || args[0] == "-h") {
		usageText(stdout)
		return 0
	}
	rest, recursive, verbose, changedOnly, reference, err := parse(args)
	if err != nil {
		fmt.Fprintf(stderr, "chmod: %v\n", err)
		tryText(stderr)
		return 1
	}
	c := command(stdin, stdout, stderr, recursive, verbose, changedOnly, reference)
	if err := c.run(rest...); err != nil {
		if errors.Is(err, errBadUsage) {
			if len(rest) > 0 {
				fmt.Fprintf(stderr, "chmod: missing operand after '%s'\n", rest[0])
			} else {
				fmt.Fprintln(stderr, "chmod: missing operand")
			}
			tryText(stderr)
			return 1
		}
		fmt.Fprintln(stderr, "chmod:", err)
		tryText(stderr)
		return 1
	}
	return 0
}
