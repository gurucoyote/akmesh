// Copyright 2017-2018 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !plan9

// id displays the user id, group id, and groups of the calling process.
//
// Synopsis:
//
//	id [-gGnu]
//
// Description:
//
//	id displays the uid, gid and groups of the calling process
//
// Options:
//
//	-g, --group     print only the effective group ID
//	-G, --groups    print all group IDs
//	-n, --name      print a name instead of a number, for -ugG
//	-u, --user      print only the effective user ID
//	-r, --user      print real ID instead of effective ID
//
// Forked from u-root 869f00f6b224ce02b1db51806159d6cd9c246f76, cmds/core/id/id.go.
// Local changes are listed below this note; the rest of this file is the
// upstream source and keeps its copyright.
package id

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

const (
	groupFile  = "/etc/group"
	passwdFile = "/etc/passwd"
)

type flags struct {
	group  bool
	groups bool
	name   bool
	user   bool
	real   bool
}

var (
	errOnlyOneChoice     = errors.New("id: cannot print \"only\" of more than one choice")
	errNotOnlyNames      = errors.New("id: cannot print only names in default format")
	errNotOnlyNamesOrIDs = errors.New("id: cannot print only names or real IDs in default format")
)

func correctFlags(flags ...bool) bool {
	n := 0
	for _, v := range flags {
		if v {
			n++
		}
	}
	return n <= 1
}

// User contains user information, as from /etc/passwd
type User struct {
	groups map[int]string
	name   string
	uid    int
	gid    int
}

// UID returns the integer UID for a user
func (u *User) UID() int {
	return u.uid
}

// GID returns the integer GID for a user
func (u *User) GID() int {
	return u.gid
}

// Name returns the name for a user
func (u *User) Name() string {
	return u.name
}

// Groups returns all the groups for a user in a map
func (u *User) Groups() map[int]string {
	return u.groups
}

// GIDName returns the group name for a user's GID
func (u *User) GIDName() string {
	return u.Groups()[u.GID()]
}

// NewUser is a factory method for the User type.
func NewUser(flags *flags, username string, users *Users, groups *Groups) (*User, error) {
	var groupsNumbers []int

	u := &User{groups: make(map[int]string)}
	if len(username) == 0 { // no username provided, get current
		if flags.real {
			u.uid = syscall.Getuid()
			u.gid = syscall.Getgid()
		} else {
			u.uid = syscall.Geteuid()
			u.gid = syscall.Getegid()
		}
		groupsNumbers, _ = syscall.Getgroups()
		// Local change: GNU id lists the primary group as well. A process whose
		// process list carries no supplementary groups would otherwise print
		// "groups=" with nothing after it.
		if !hasGroup(groupsNumbers, u.gid) {
			groupsNumbers = append([]int{u.gid}, groupsNumbers...)
		}
		if v, err := users.GetUser(u.uid); err == nil {
			u.name = v
		} else {
			u.name = strconv.Itoa(u.uid)
		}
	} else {
		if v, err := users.GetUID(username); err == nil { // user is username
			u.name = username
			u.uid = v
		} else {
			if uid, err := strconv.Atoi(username); err == nil { // user is valid int
				if v, err := users.GetUser(uid); err == nil { // user is valid uid
					u.name = v
					u.uid = uid
				}
			} else {
				return nil, fmt.Errorf("id: no such user or uid: %s", username)
			}
		}
		u.gid, _ = users.GetGID(u.uid)
		groupsNumbers = append([]int{u.gid}, groups.UserGetGIDs(u.name)...)
		// FIXME: not yet implemented group listing lookups
	}

	for _, groupNum := range groupsNumbers {
		if groupName, err := groups.GetGroup(groupNum); err == nil {
			u.groups[groupNum] = groupName
		} else {
			u.groups[groupNum] = strconv.Itoa(groupNum)
		}
	}

	return u, nil
}

// IDCommand runs the "id" with the current user's information. It reports
// whether a name was asked for (-n) but missing, which GNU treats as an error
// after printing the number.
func IDCommand(w, errw io.Writer, flags *flags, u User) (missingName bool) {
	if !flags.groups {
		if flags.user {
			if flags.name {
				if !knownName(u.Name(), u.UID()) {
					fmt.Fprintf(errw, "id: cannot find name for user ID %d\n", u.UID())
					missingName = true
				}
				fmt.Fprintln(w, u.Name())
				return
			}
			fmt.Fprintln(w, u.UID())
			return
		} else if flags.group {
			if flags.name {
				if !knownName(u.GIDName(), u.GID()) {
					fmt.Fprintf(errw, "id: cannot find name for group ID %d\n", u.GID())
					missingName = true
				}
				fmt.Fprintln(w, u.GIDName())
				return
			}
			fmt.Fprintln(w, u.GID())
			return
		}

		if knownName(u.Name(), u.UID()) {
			fmt.Fprintf(w, "uid=%d(%s) ", u.UID(), u.Name())
		} else {
			fmt.Fprintf(w, "uid=%d ", u.UID())
		}
		if knownName(u.GIDName(), u.GID()) {
			fmt.Fprintf(w, "gid=%d(%s) ", u.GID(), u.GIDName())
		} else {
			fmt.Fprintf(w, "gid=%d ", u.GID())
		}
	}

	if !flags.groups {
		fmt.Fprintf(w, "groups=")
	}

	// handle Go map ordering
	var gids []int
	for gid := range u.Groups() {
		gids = append(gids, gid)
	}
	sort.Ints(gids)

	var groupOutput []string
	for _, id := range gids {
		gid, name := id, u.Groups()[id]
		if !flags.groups {
			if knownName(name, gid) {
				groupOutput = append(groupOutput, fmt.Sprintf("%d(%s)", gid, name))
			} else {
				groupOutput = append(groupOutput, fmt.Sprintf("%d", gid))
			}
		} else {
			if flags.name {
				groupOutput = append(groupOutput, fmt.Sprintf("%s ", name))
			} else {
				groupOutput = append(groupOutput, fmt.Sprintf("%d ", gid))
			}
		}
	}

	sep := ","
	if flags.groups {
		sep = ""
	}

	fmt.Fprintln(w, strings.Join(groupOutput, sep))
	return missingName
}

func hasGroup(groups []int, gid int) bool {
	for _, g := range groups {
		if g == gid {
			return true
		}
	}
	return false
}

// knownName says whether the database knew the name. Without /etc/passwd the
// name field carries the number itself, which GNU prints without parentheses.
func knownName(name string, num int) bool {
	return name != "" && name != strconv.Itoa(num)
}

func run(w, errw io.Writer, name string, f *flags, passwd, group string) (bool, error) {
	if !correctFlags(f.groups, f.group, f.user) {
		return false, errOnlyOneChoice
	}
	if f.name && (!f.groups && !f.group && !f.user) {
		return false, errNotOnlyNames
	}
	if len(name) != 0 && f.real {
		return false, errNotOnlyNamesOrIDs
	}

	users, err := NewUsers(passwd)
	if err != nil {
		return false, fmt.Errorf("id: %w", err)
	}
	groups, err := NewGroups(group)
	if err != nil {
		return false, fmt.Errorf("id: %w", err)
	}

	user, err := NewUser(f, name, users, groups)
	if err != nil {
		return false, fmt.Errorf("id: %w", err)
	}

	return IDCommand(w, errw, f, *user), nil
}

// expandShort splits combined short flags, as in id -un: GNU id and children
// that type it do not spell out -u -n, and Go's flag package does not either.
// Only letters this command knows are split, so an unknown flag still reaches
// the parser as one word.
func expandShort(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		if len(a) < 3 || a[0] != '-' || a[1] == '-' {
			out = append(out, a)
			continue
		}
		all := true
		for _, c := range a[1:] {
			if !strings.ContainsRune("gGnur", c) {
				all = false
				break
			}
		}
		if !all {
			out = append(out, a)
			continue
		}
		for _, c := range a[1:] {
			out = append(out, "-"+string(c))
		}
	}
	return out
}

// Run is the command line of id, reachable from the akmesh binary.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	f := &flags{}
	fs := flag.NewFlagSet("id", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.BoolVar(&f.group, "g", false, "print only the effective group ID")
	fs.BoolVar(&f.groups, "G", false, "print all group IDs")
	fs.BoolVar(&f.name, "n", false, "print a name instead of a number, for -ugG")
	fs.BoolVar(&f.user, "u", false, "print only the effective user ID")
	fs.BoolVar(&f.real, "r", false, "print real ID instead of effective ID")
	if err := fs.Parse(expandShort(args)); err != nil {
		return 2
	}
	missingName, err := run(stdout, stderr, fs.Arg(0), f, passwdFile, groupFile)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	if missingName {
		return 1
	}
	return 0
}
