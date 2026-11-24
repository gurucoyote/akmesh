// Package userdb reads and writes the account files: /etc/passwd, /etc/group,
// /etc/shadow and /etc/gshadow. The two families that use it differ in their
// defaults and in their messages, so this package holds only what both need:
// parsing that keeps unknown lines, id ranges from /etc/login.defs, and an
// append that never rewrites the whole file.
package userdb

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// The paths are variables: a test points them at a throwaway copy. AKMESH_ROOT
// prefixes them, the way apk installs into a tree that is not / - without it a
// package script under a foreign root would edit the host's accounts.
var (
	PasswdFile  = "/etc/passwd"
	GroupFile   = "/etc/group"
	ShadowFile  = "/etc/shadow"
	GShadowFile = "/etc/gshadow"
	LoginDefs   = "/etc/login.defs"
)

func init() {
	if root := os.Getenv("AKMESH_ROOT"); root != "" {
		prefix := strings.TrimSuffix(root, "/")
		PasswdFile = prefix + "/etc/passwd"
		GroupFile = prefix + "/etc/group"
		ShadowFile = prefix + "/etc/shadow"
		GShadowFile = prefix + "/etc/gshadow"
		LoginDefs = prefix + "/etc/login.defs"
	}
}

// User is one line of /etc/passwd.
type User struct {
	Name   string
	Passwd string
	UID    int
	GID    int
	Gecos  string
	Home   string
	Shell  string
}

// Group is one line of /etc/group.
type Group struct {
	Name    string
	Passwd  string
	GID     int
	Members []string
}

// Ranges are the id windows from /etc/login.defs with the shadow defaults.
type Ranges struct {
	SysUIDMin, SysUIDMax int
	UIDMin, UIDMax       int
	SysGIDMin, SysGIDMax int
	GIDMin, GIDMax       int
}

// DefaultRanges is what login.defs says when the file is missing or silent;
// these are shadow's own defaults, and busybox uses the same system window.
func DefaultRanges() Ranges {
	return Ranges{
		SysUIDMin: 100, SysUIDMax: 999,
		UIDMin: 1000, UIDMax: 60000,
		SysGIDMin: 100, SysGIDMax: 999,
		GIDMin: 1000, GIDMax: 60000,
	}
}

// LoadRanges reads login.defs. A missing file is not an error: the defaults
// are what a fresh system would use anyway.
func LoadRanges() Ranges {
	r := DefaultRanges()
	data, err := os.ReadFile(LoginDefs)
	if err != nil {
		return r
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		switch fields[0] {
		case "SYS_UID_MIN":
			r.SysUIDMin = n
		case "SYS_UID_MAX":
			r.SysUIDMax = n
		case "UID_MIN":
			r.UIDMin = n
		case "UID_MAX":
			r.UIDMax = n
		case "SYS_GID_MIN":
			r.SysGIDMin = n
		case "SYS_GID_MAX":
			r.SysGIDMax = n
		case "GID_MIN":
			r.GIDMin = n
		case "GID_MAX":
			r.GIDMax = n
		}
	}
	return r
}

// Users parses /etc/passwd. A missing file is empty, not an error: an image
// without accounts is still an image.
func Users() ([]User, error) {
	var users []User
	err := eachLine(PasswdFile, func(line string) error {
		f := strings.Split(line, ":")
		if len(f) < 7 {
			return fmt.Errorf("malformed line %q in %s", line, PasswdFile)
		}
		uid, err := strconv.Atoi(f[2])
		if err != nil {
			return fmt.Errorf("bad uid %q in %s", f[2], PasswdFile)
		}
		gid, err := strconv.Atoi(f[3])
		if err != nil {
			return fmt.Errorf("bad gid %q in %s", f[3], PasswdFile)
		}
		users = append(users, User{
			Name: f[0], Passwd: f[1], UID: uid, GID: gid,
			Gecos: f[4], Home: f[5], Shell: f[6],
		})
		return nil
	})
	return users, err
}

// Groups parses /etc/group.
func Groups() ([]Group, error) {
	var groups []Group
	err := eachLine(GroupFile, func(line string) error {
		f := strings.Split(line, ":")
		if len(f) < 4 {
			return fmt.Errorf("malformed line %q in %s", line, GroupFile)
		}
		gid, err := strconv.Atoi(f[2])
		if err != nil {
			return fmt.Errorf("bad gid %q in %s", f[2], GroupFile)
		}
		var members []string
		for _, m := range strings.Split(f[3], ",") {
			if m != "" {
				members = append(members, m)
			}
		}
		groups = append(groups, Group{Name: f[0], Passwd: f[1], GID: gid, Members: members})
		return nil
	})
	return groups, err
}

// FindUser looks a name up, and an id when the string is a number. The bool
// distinguishes "not there" from "could not read".
func FindUser(name string) (User, bool, error) {
	users, err := Users()
	if err != nil {
		return User{}, false, err
	}
	for _, u := range users {
		if u.Name == name {
			return u, true, nil
		}
	}
	if id, err := strconv.Atoi(name); err == nil {
		for _, u := range users {
			if u.UID == id {
				return u, true, nil
			}
		}
	}
	return User{}, false, nil
}

// FindGroup looks a group up by name or gid.
func FindGroup(name string) (Group, bool, error) {
	groups, err := Groups()
	if err != nil {
		return Group{}, false, err
	}
	for _, g := range groups {
		if g.Name == name {
			return g, true, nil
		}
	}
	if id, err := strconv.Atoi(name); err == nil {
		for _, g := range groups {
			if g.GID == id {
				return g, true, nil
			}
		}
	}
	return Group{}, false, nil
}

// FindGroupByID is the lookup userdel needs: who owns gid N.
func FindGroupByID(gid int) (Group, bool, error) {
	groups, err := Groups()
	if err != nil {
		return Group{}, false, err
	}
	for _, g := range groups {
		if g.GID == gid {
			return g, true, nil
		}
	}
	return Group{}, false, nil
}

// NextSystemID takes the highest id in the system window plus one. Busybox does
// NOT do this (it takes the lowest free id, see NextFreeID); the function stays
// for tools that walk a range from the top.
func NextSystemID(min, max int, used map[int]bool) (int, error) {
	next := min - 1
	for id := range used {
		if id >= min && id <= max && id > next {
			next = id
		}
	}
	next++
	if next > max {
		return 0, fmt.Errorf("no free id in %d..%d", min, max)
	}
	return next, nil
}

// FreeSystemIDFromTop takes the highest free id, the way shadow's useradd and
// groupadd do with -r: on a fresh Debian the first system account gets 999.
func FreeSystemIDFromTop(min, max int, used map[int]bool) (int, error) {
	for id := max; id >= min; id-- {
		if !used[id] {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no free id in %d..%d", min, max)
}

// NextFreeID takes the lowest free id in a window, shadow's choice for
// ordinary accounts.
func NextFreeID(min, max int, used map[int]bool) (int, error) {
	for id := min; id <= max; id++ {
		if !used[id] {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no free id in %d..%d", min, max)
}

// AppendPasswd adds a line and keeps the file's mode. Passwd must already be a
// valid passwd line: formatting is the caller's business, because the two
// families write different shadow fields.
func AppendPasswd(line string) error { return appendLine(PasswdFile, line) }

// AppendGroup adds a group line.
func AppendGroup(line string) error { return appendLine(GroupFile, line) }

// AppendShadow adds a shadow line, if the system has a shadow file at all. A
// system without one stores the password in /etc/passwd, and creating the file
// behind its back would lock every account out.
func AppendShadow(line string) error { return appendIfExists(ShadowFile, line) }

// AppendGShadow adds a gshadow line the same way.
func AppendGShadow(line string) error { return appendIfExists(GShadowFile, line) }

func appendIfExists(path, line string) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	return appendLine(path, line)
}

// appendLine adds a line with a final newline, whether or not the file ended
// with one. Adding is the normal case, so the file is rewritten only when it
// has no trailing newline.
func appendLine(path, line string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	if st.Size() > 0 {
		last := make([]byte, 1)
		if _, err := f.ReadAt(last, st.Size()-1); err != nil {
			return err
		}
		if last[0] != '\n' {
			if _, err := f.WriteString("\n"); err != nil {
				return err
			}
		}
	}
	_, err = f.WriteString(line + "\n")
	return err
}

// RemoveFromFile drops every line that drop matches and returns how many lines
// went. The rewrite goes through a temporary file in the same directory, so a
// full disk leaves the original in place; mode and owner are carried over.
func RemoveFromFile(path string, drop func(line string) bool) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	st, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	var kept []string
	removed := 0
	for _, line := range strings.Split(string(data), "\n") {
		if line != "" && drop(line) {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if removed == 0 {
		return 0, nil
	}
	body := strings.Join(kept, "\n")
	if !strings.HasSuffix(body, "\n") && body != "" {
		body += "\n"
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".userdb-*")
	if err != nil {
		return 0, err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(st.Mode().Perm()); err != nil {
		tmp.Close()
		return 0, err
	}
	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		return 0, err
	}
	if err := tmp.Close(); err != nil {
		return 0, err
	}
	if err := os.Rename(name, path); err != nil {
		return 0, err
	}
	return removed, nil
}

// RemoveLine removes a line whose first field is name from one of the account
// files. It reports whether a line was there.
func RemoveLine(path, name string) (bool, error) {
	// A system without gshadow (busybox, alpine) has nothing to remove, and
	// that is not an error: the caller asked for the line to be gone.
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	n, err := RemoveFromFile(path, func(line string) bool {
		f := strings.SplitN(line, ":", 2)
		return f[0] == name
	})
	return n > 0, err
}

// AddMember appends name to the member list of group g in /etc/group.
func AddMember(group, name string) error {
	return editFile(GroupFile, func(line string) (string, error) {
		f := strings.Split(line, ":")
		if len(f) < 4 || f[0] != group {
			return line, nil
		}
		for _, m := range strings.Split(f[3], ",") {
			if m == name {
				return line, nil
			}
		}
		if f[3] == "" {
			f[3] = name
		} else {
			f[3] += "," + name
		}
		return strings.Join(f, ":"), nil
	})
}

// DropMember removes name from the member list of group g.
func DropMember(group, name string) error {
	return editFile(GroupFile, func(line string) (string, error) {
		f := strings.Split(line, ":")
		if len(f) < 4 || f[0] != group {
			return line, nil
		}
		var kept []string
		for _, m := range strings.Split(f[3], ",") {
			if m != "" && m != name {
				kept = append(kept, m)
			}
		}
		f[3] = strings.Join(kept, ",")
		return strings.Join(f, ":"), nil
	})
}

// editFile rewrites every line through fn, and only if something changed.
func editFile(path string, fn func(string) (string, error)) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	st, err := os.Stat(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	changed := false
	for i, line := range lines {
		if line == "" {
			continue
		}
		next, err := fn(line)
		if err != nil {
			return err
		}
		if next != line {
			lines[i] = next
			changed = true
		}
	}
	if !changed {
		return nil
	}
	body := strings.Join(lines, "\n")
	tmp, err := os.CreateTemp(filepath.Dir(path), ".userdb-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(st.Mode().Perm()); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

// eachLine walks a colon file, skipping blank lines and comments.
func eachLine(path string, fn func(string) error) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := fn(line); err != nil {
			return err
		}
	}
	return sc.Err()
}

// ShadowLine is the field order of /etc/shadow. Busybox writes the age fields,
// shadow's useradd leaves them empty, so the caller decides.
type ShadowLine struct {
	Name    string
	Passwd  string
	LastChg int64
	Rest    string
}

// DaysSinceEpoch is what the shadow field wants: days, not seconds.
func DaysSinceEpoch(t time.Time) int64 {
	return t.UTC().Unix() / 86400
}

// SortedNames is for messages that list members in a stable order.
func SortedNames(names []string) []string {
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out
}
