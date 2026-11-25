//go:build !windows

// Package chown changes the owner and the group of files, as agents expect it
// from every Unix: unpacking an archive or cleaning up after a service needs it,
// and without it they reach for sudo or python (acmesh-6p8).
package chown

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Run implements chown [-R] [-h] [-v] [-c] [-f] OWNER[:GROUP] FILE...
//
// OWNER may be a name or a number; ":GROUP", "OWNER.GROUP" and a bare ":GROUP"
// (group only) are understood, and a missing part is left unchanged, which is
// what chown's -1 means.
func Run(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	recursive := false
	noDeref := false
	verbose := false
	changes := false
	silent := false
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-R" || a == "--recursive":
			recursive = true
		case a == "-h" || a == "--no-dereference":
			noDeref = true
		case a == "-v" || a == "--verbose":
			verbose = true
		case a == "-c" || a == "--changes":
			changes = true
		case a == "-f" || a == "--silent" || a == "--quiet":
			silent = true
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
		case len(a) > 1 && a[0] == '-':
			fmt.Fprintf(stderr, "chown: unrecognized option '%s'\n", a)
			fmt.Fprintln(stderr, "Try 'chown --help' for more information.")
			return 1
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) < 2 {
		fmt.Fprintln(stderr, "chown: usage: chown [-R] [-h] [-v] OWNER[:GROUP] FILE...")
		return 1
	}
	uid, gid, err := parseOwner(rest[0])
	if err != nil {
		fmt.Fprintf(stderr, "chown: %v\n", err)
		return 1
	}
	code := 0
	change := func(path string) {
		// -c prints only when the ownership really changed; -v always prints.
		var beforeUID, beforeGID int
		hadOwner := false
		if changes && !verbose {
			if fi, err := os.Lstat(path); err == nil {
				if st, ok := fi.Sys().(*syscall.Stat_t); ok {
					beforeUID, beforeGID, hadOwner = int(st.Uid), int(st.Gid), true
				}
			}
		}
		var err error
		if noDeref {
			err = os.Lchown(path, uid, gid)
		} else {
			err = os.Chown(path, uid, gid)
		}
		if err != nil {
			if !silent {
				fmt.Fprintf(stderr, "chown: %v\n", err)
			}
			code = 1
			return
		}
		show := verbose
		if !verbose && hadOwner {
			show = (uid != -1 && uid != beforeUID) || (gid != -1 && gid != beforeGID)
		}
		if show {
			fmt.Fprintf(stdout, "changed ownership of '%s'\n", path)
		}
	}
	for _, target := range rest[1:] {
		if !recursive {
			change(target)
			continue
		}
		err := filepath.WalkDir(target, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				if !silent {
					fmt.Fprintf(stderr, "chown: %v\n", err)
				}
				code = 1
				return nil
			}
			change(path)
			return nil
		})
		if err != nil && !silent {
			fmt.Fprintf(stderr, "chown: %v\n", err)
			code = 1
		}
	}
	return code
}

// parseOwner reads "user", "user:group", "user.group" or ":group".
func parseOwner(spec string) (int, int, error) {
	uid, gid := -1, -1
	user, group := spec, ""
	if i := strings.IndexAny(spec, ":."); i >= 0 {
		user, group = spec[:i], spec[i+1:]
	}
	if user != "" {
		n, err := lookupUser(user)
		if err != nil {
			return 0, 0, err
		}
		uid = n
	}
	if group != "" {
		n, err := lookupGroup(group)
		if err != nil {
			return 0, 0, err
		}
		gid = n
	}
	if uid == -1 && gid == -1 {
		return 0, 0, fmt.Errorf("invalid owner '%s'", spec)
	}
	return uid, gid, nil
}

// lookupUser accepts a name from /etc/passwd or a plain number.
func lookupUser(s string) (int, error) {
	if n, err := strconv.Atoi(s); err == nil {
		if n < 0 {
			return 0, fmt.Errorf("invalid user '%s'", s)
		}
		return n, nil
	}
	data, err := os.ReadFile("/etc/passwd")
	if err != nil {
		return 0, fmt.Errorf("invalid user '%s': cannot read /etc/passwd: %v", s, err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		f := strings.Split(line, ":")
		if len(f) < 4 || f[0] != s {
			continue
		}
		if n, err := strconv.Atoi(f[2]); err == nil {
			return n, nil
		}
	}
	return 0, fmt.Errorf("invalid user '%s'", s)
}

// lookupGroup accepts a name from /etc/group or a plain number.
func lookupGroup(s string) (int, error) {
	if n, err := strconv.Atoi(s); err == nil {
		if n < 0 {
			return 0, fmt.Errorf("invalid group '%s'", s)
		}
		return n, nil
	}
	data, err := os.ReadFile("/etc/group")
	if err != nil {
		return 0, fmt.Errorf("invalid group '%s': cannot read /etc/group: %v", s, err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		f := strings.Split(line, ":")
		if len(f) < 3 || f[0] != s {
			continue
		}
		if n, err := strconv.Atoi(f[2]); err == nil {
			return n, nil
		}
	}
	return 0, fmt.Errorf("invalid group '%s'", s)
}
