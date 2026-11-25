// Package useradd implements the account commands the way busybox does it,
// because that is what Alpine carries: adduser, addgroup, deluser and delgroup.
// The GNU shadow tools (useradd, groupadd, userdel, groupdel) are not built:
// on Alpine base they do not exist, so an agent that works there types adduser.
//
// The measured behaviour this code follows (busybox 1.37.0 on alpine:3.22,
// 24.11.2026) is in docs/measurements.md, section "Benutzerwerkzeuge".
package useradd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"akmesh/internal/userdb"
)

// Group membership messages name the command that was called; busybox prints
// "addgroup:" even from adduser, which is a quirk and not worth copying.
type runner struct {
	cmd    string
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

// Run answers adduser, addgroup, deluser and delgroup.
func Run(cmd string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if runtime.GOOS == "windows" {
		fmt.Fprintf(stderr, "%s: not supported on Windows (there is no /etc/passwd)\n", cmd)
		return 1
	}
	r := &runner{cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr}
	var err error
	switch cmd {
	case "adduser":
		err = r.addUser(args)
	case "addgroup":
		err = r.addGroup(args)
	case "deluser":
		err = r.delUser(args)
	case "delgroup":
		err = r.delGroup(args)
	default:
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", cmd, err)
		return 1
	}
	return 0
}

func (r *runner) addUser(args []string) error {
	var (
		home      string
		gecos     string
		shell     string
		group     string
		uidFlag   string
		skel      = "/etc/skel"
		system    bool
		noPasswd  bool
		noHomeDir bool
	)
	rest, err := parseFlags(args, map[byte]bool{
		'h': true, 'g': true, 's': true, 'G': true, 'u': true, 'k': true,
		'S': false, 'D': false, 'H': false,
	}, func(flag byte, val string) {
		switch flag {
		case 'h':
			home = val
		case 'g':
			gecos = val
		case 's':
			shell = val
		case 'G':
			group = val
		case 'u':
			uidFlag = val
		case 'k':
			skel = val
		case 'S':
			system = true
		case 'D':
			noPasswd = true
		case 'H':
			noHomeDir = true
		}
	})
	if err != nil {
		return err
	}

	// Two operands: add an existing user to an existing group.
	if len(rest) == 2 {
		return r.addMember(rest[0], rest[1])
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: adduser [OPTIONS] USER [GROUP]")
	}
	name := rest[0]

	if _, found, err := userdb.FindUser(name); err != nil {
		return err
	} else if found {
		return fmt.Errorf("user '%s' in use", name)
	}
	ranges := userdb.LoadRanges()
	users, err := userdb.Users()
	if err != nil {
		return err
	}
	groups, err := userdb.Groups()
	if err != nil {
		return err
	}
	usedUID := map[int]bool{}
	usedGID := map[int]bool{}
	for _, u := range users {
		usedUID[u.UID] = true
		usedGID[u.GID] = true
	}
	for _, g := range groups {
		usedGID[g.GID] = true
	}

	uid := 0
	if uidFlag != "" {
		uid, err = strconv.Atoi(uidFlag)
		if err != nil {
			return fmt.Errorf("invalid uid '%s'", uidFlag)
		}
		if usedUID[uid] {
			return fmt.Errorf("uid '%d' in use", uid)
		}
	} else if system {
		// Busybox takes the LOWEST free id in the system window, not the next
		// after the highest one (measured on alpine: 100 taken gives 101).
		uid, err = userdb.NextFreeID(ranges.SysUIDMin, ranges.SysUIDMax, usedUID)
		if err != nil {
			return err
		}
	} else {
		uid, err = userdb.NextFreeID(ranges.UIDMin, ranges.UIDMax, usedUID)
		if err != nil {
			return err
		}
	}

	// Group: -G wins; otherwise a group of the same name is created, and the
	// user's gid follows the uid when that gid is free (busybox).
	gid := uid
	createGroup := true
	if group != "" {
		g, found, err := userdb.FindGroup(group)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("group '%s' does not exist", group)
		}
		gid = g.GID
		createGroup = false
	} else if g, found, err := userdb.FindGroup(name); err != nil {
		return err
	} else if found {
		gid = g.GID
		createGroup = false
	} else if usedGID[gid] {
		gid, err = userdb.NextFreeID(ranges.GIDMin, ranges.GIDMax, usedGID)
		if err != nil {
			return err
		}
	}

	if home == "" {
		home = "/home/" + name
	}
	if shell == "" {
		shell = "/bin/sh"
	}

	passwdField := "x"
	shadowLine := ""
	if _, err := os.Stat(userdb.ShadowFile); err != nil {
		passwdField = "!"
	} else {
		shadowLine = fmt.Sprintf("%s:%s:%d:0:99999:7:::",
			name, passwordField(noPasswd, r.stdin, r.stdout, name),
			userdb.DaysSinceEpoch(time.Now()))
	}

	if createGroup {
		if err := userdb.AppendGroup(fmt.Sprintf("%s:x:%d:", name, gid)); err != nil {
			return err
		}
		if err := userdb.AppendGShadow(fmt.Sprintf("%s:!::", name)); err != nil {
			return err
		}
	} else if group != "" {
		// busybox lists the new account in the group it joins, not only as its
		// gid: measured, `adduser ... -G sysa` gives "sysa:x:101:svc2".
		if err := userdb.AddMember(group, name); err != nil {
			return err
		}
	}
	if !noHomeDir {
		if err := makeHome(home, skel, uid, gid); err != nil {
			return err
		}
	}
	line := fmt.Sprintf("%s:%s:%d:%d:%s:%s:%s", name, passwdField, uid, gid, gecos, home, shell)
	if err := userdb.AppendPasswd(line); err != nil {
		return err
	}
	if shadowLine != "" {
		if err := userdb.AppendShadow(shadowLine); err != nil {
			return err
		}
	}
	return nil
}

// passwordField is "!" with -D, and without it a locked field unless someone
// can type: busybox asks passwd here, and a prompt on a pipe would hang.
func passwordField(noPasswd bool, stdin io.Reader, stdout io.Writer, name string) string {
	if noPasswd {
		return "!"
	}
	if isTerminal(stdin) && isTerminal(stdout) {
		// The password prompt is passwd's job; adduser only reserves the line.
		return "!"
	}
	return "!"
}

func (r *runner) addGroup(args []string) error {
	var (
		gidFlag string
		system  bool
	)
	rest, err := parseFlags(args, map[byte]bool{'g': true, 'S': false}, func(flag byte, val string) {
		switch flag {
		case 'g':
			gidFlag = val
		case 'S':
			system = true
		}
	})
	if err != nil {
		return err
	}
	if len(rest) == 2 {
		return r.addMember(rest[0], rest[1])
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: addgroup [-g GID] [-S] [USER] GROUP")
	}
	name := rest[0]
	if _, found, err := userdb.FindGroup(name); err != nil {
		return err
	} else if found {
		return fmt.Errorf("group '%s' in use", name)
	}
	groups, err := userdb.Groups()
	if err != nil {
		return err
	}
	used := map[int]bool{}
	for _, g := range groups {
		used[g.GID] = true
	}
	ranges := userdb.LoadRanges()

	gid := 0
	if gidFlag != "" {
		gid, err = strconv.Atoi(gidFlag)
		if err != nil {
			return fmt.Errorf("invalid gid '%s'", gidFlag)
		}
		if used[gid] {
			return fmt.Errorf("gid '%d' in use", gid)
		}
	} else if system {
		gid, err = userdb.NextFreeID(ranges.SysGIDMin, ranges.SysGIDMax, used)
		if err != nil {
			return err
		}
	} else {
		gid, err = userdb.NextFreeID(ranges.GIDMin, ranges.GIDMax, used)
		if err != nil {
			return err
		}
	}
	if err := userdb.AppendGroup(fmt.Sprintf("%s:x:%d:", name, gid)); err != nil {
		return err
	}
	return userdb.AppendGShadow(fmt.Sprintf("%s:!::", name))
}

// addMember is the two-operand form of both add commands: USER GROUP.
func (r *runner) addMember(user, group string) error {
	if _, found, err := userdb.FindGroup(group); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("unknown group %s", group)
	}
	if _, found, err := userdb.FindUser(user); err != nil {
		return err
	} else if !found {
		return fmt.Errorf("unknown user %s", user)
	}
	return userdb.AddMember(group, user)
}

func (r *runner) delUser(args []string) error {
	removeHome := false
	var rest []string
	for _, a := range args {
		switch a {
		case "--remove-home", "-r":
			removeHome = true
		default:
			if strings.HasPrefix(a, "-") {
				return fmt.Errorf("unrecognized option '%s'", a)
			}
			rest = append(rest, a)
		}
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: deluser [--remove-home] USER")
	}
	name := rest[0]
	user, found, err := userdb.FindUser(name)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("unknown user %s", name)
	}
	if _, err := userdb.RemoveLine(userdb.PasswdFile, name); err != nil {
		return err
	}
	if _, err := userdb.RemoveLine(userdb.ShadowFile, name); err != nil {
		return err
	}
	if _, err := userdb.RemoveLine(userdb.GShadowFile, name); err != nil {
		return err
	}
	// The user leaves every member list, and the group that belongs to them
	// goes with them: busybox removed the u1 group in the measured run.
	groups, err := userdb.Groups()
	if err != nil {
		return err
	}
	for _, g := range groups {
		hasMember := false
		for _, m := range g.Members {
			if m == name {
				hasMember = true
			}
		}
		if hasMember {
			if err := userdb.DropMember(g.Name, name); err != nil {
				return err
			}
		}
		if g.Name == name && g.GID == user.GID {
			if _, err := userdb.RemoveLine(userdb.GroupFile, g.Name); err != nil {
				return err
			}
		}
	}
	if removeHome {
		if err := os.RemoveAll(user.Home); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) delGroup(args []string) error {
	var rest []string
	for _, a := range args {
		if strings.HasPrefix(a, "-") {
			return fmt.Errorf("unrecognized option '%s'", a)
		}
		rest = append(rest, a)
	}
	switch len(rest) {
	case 1:
		group := rest[0]
		g, found, err := userdb.FindGroup(group)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("unknown group %s", group)
		}
		users, err := userdb.Users()
		if err != nil {
			return err
		}
		for _, u := range users {
			if u.GID == g.GID {
				//lint:ignore ST1005 GNU-Wortlaut mit Ausrufezeichen, absichtlich so gedruckt
				return fmt.Errorf("'%s' still has '%s' as their primary group!", u.Name, group)
			}
		}
		if _, err := userdb.RemoveLine(userdb.GroupFile, group); err != nil {
			return err
		}
		_, err = userdb.RemoveLine(userdb.GShadowFile, group)
		return err
	case 2:
		user, group := rest[0], rest[1]
		if _, found, err := userdb.FindGroup(group); err != nil {
			return err
		} else if !found {
			return fmt.Errorf("unknown group %s", group)
		}
		if _, found, err := userdb.FindUser(user); err != nil {
			return err
		} else if !found {
			return fmt.Errorf("unknown user %s", user)
		}
		return userdb.DropMember(group, user)
	default:
		return fmt.Errorf("usage: delgroup [USER] GROUP")
	}
}

// makeHome creates the home directory and copies the skeleton into it, owned
// by the new account. A missing skeleton is not an error: alpine has none.
func makeHome(home, skel string, uid, gid int) error {
	if err := os.MkdirAll(home, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(skel)
	if err != nil {
		if os.IsNotExist(err) {
			return os.Chown(home, uid, gid)
		}
		return err
	}
	for _, e := range entries {
		src := filepath.Join(skel, e.Name())
		dst := filepath.Join(home, e.Name())
		if err := copyTree(src, dst, uid, gid); err != nil {
			return err
		}
	}
	return nil
}

func copyTree(src, dst string, uid, gid int) error {
	st, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if st.IsDir() {
		if err := os.MkdirAll(dst, st.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyTree(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()), uid, gid); err != nil {
				return err
			}
		}
		return os.Chown(dst, uid, gid)
	}
	if st.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(src)
		if err != nil {
			return err
		}
		if err := os.Symlink(target, dst); err != nil {
			return err
		}
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, st.Mode().Perm()); err != nil {
		return err
	}
	return os.Chown(dst, uid, gid)
}

// parseFlags reads busybox-style short options, including the combined form
// (-SD, -SDH) that agents type. Long options are not part of these commands.
func parseFlags(args []string, takesValue map[byte]bool, set func(flag byte, val string)) ([]string, error) {
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i+1:]...)
			break
		}
		if len(a) < 2 || a[0] != '-' {
			rest = append(rest, a)
			continue
		}
		for j := 1; j < len(a); j++ {
			flag := a[j]
			wantsValue, known := takesValue[flag]
			if !known {
				return nil, fmt.Errorf("unrecognized option '-%c'", flag)
			}
			if !wantsValue {
				set(flag, "")
				continue
			}
			// A value either follows in the same word (-gmain) or in the next.
			if j+1 < len(a) {
				set(flag, a[j+1:])
				j = len(a)
				continue
			}
			if i+1 >= len(args) {
				return nil, fmt.Errorf("option '-%c' requires an argument", flag)
			}
			i++
			set(flag, args[i])
			break
		}
	}
	return rest, nil
}

// isTerminal reports whether r is a terminal file, which decides whether a
// password prompt would reach a human.
func isTerminal(v any) bool {
	f, ok := v.(*os.File)
	if !ok {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}
