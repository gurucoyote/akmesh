package useradd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gurucoyote/akmesh/internal/userdb"
)

// fakeEtc points the package at a throwaway tree and fills it with the base
// accounts an image has.
func fakeEtc(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, "etc", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("passwd", "root:x:0:0:root:/root:/bin/sh\n")
	write("group", "root:x:0:\nwheel:x:100:\n")
	write("shadow", "root:!:20781:0:99999:7:::\n")
	old := []*string{&userdb.PasswdFile, &userdb.GroupFile, &userdb.ShadowFile, &userdb.GShadowFile, &userdb.LoginDefs}
	userdb.PasswdFile = filepath.Join(dir, "etc", "passwd")
	userdb.GroupFile = filepath.Join(dir, "etc", "group")
	userdb.ShadowFile = filepath.Join(dir, "etc", "shadow")
	userdb.GShadowFile = filepath.Join(dir, "etc", "gshadow")
	userdb.LoginDefs = filepath.Join(dir, "etc", "login.defs")
	t.Cleanup(func() {
		userdb.PasswdFile = *old[0]
		userdb.GroupFile = *old[1]
		userdb.ShadowFile = *old[2]
		userdb.GShadowFile = *old[3]
		userdb.LoginDefs = *old[4]
	})
	return dir
}

func read(t *testing.T, dir, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "etc", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// run is the test wrapper: collects stderr and the exit code.
func run(cmd string, args ...string) (int, string) {
	var out, errb bytes.Buffer
	code := Run(cmd, args, strings.NewReader(""), &out, &errb)
	return code, errb.String()
}

// TestAddGroupSystem: the system range takes the highest used id plus one, as
// busybox does (measured: 100 in a fresh alpine gives 101).
func TestAddGroupSystem(t *testing.T) {
	dir := fakeEtc(t)
	if code, err := run("addgroup", "-S", "sysg"); code != 0 {
		t.Fatalf("addgroup -S failed: %d %s", code, err)
	}
	if got := read(t, dir, "group"); !strings.Contains(got, "sysg:x:101:") {
		t.Fatalf("system gid not the next after 100:\n%s", got)
	}
	if code, err := run("addgroup", "-S", "sysg"); code == 0 || !strings.Contains(err, "group 'sysg' in use") {
		t.Fatalf("duplicate group not refused: %d %q", code, err)
	}
	if code, err := run("addgroup", "-g", "100", "other"); code == 0 || !strings.Contains(err, "gid '100' in use") {
		t.Fatalf("used gid not refused: %d %q", code, err)
	}
}

// TestAddUserLikeAPackageScript is the line from busybox .post-install and
// dbus .pre-install, the reason the ticket exists.
func TestAddUserLikeAPackageScript(t *testing.T) {
	dir := fakeEtc(t)
	if code, err := run("addgroup", "-S", "svcgrp"); code != 0 {
		t.Fatalf("addgroup: %d %s", code, err)
	}
	code, errText := run("adduser", "-S", "-D", "-H", "-h", "/dev/null", "-s", "/sbin/nologin", "-G", "svcgrp", "svc")
	if code != 0 {
		t.Fatalf("adduser failed: %d %s", code, errText)
	}
	passwd := read(t, dir, "passwd")
	if !strings.Contains(passwd, "svc:x:100:101::/dev/null:/sbin/nologin") {
		t.Fatalf("passwd line wrong:\n%s", passwd)
	}
	shadow := read(t, dir, "shadow")
	if !strings.Contains(shadow, "svc:!:") || !strings.HasSuffix(shadow, "0:99999:7:::\n") {
		t.Fatalf("shadow line wrong:\n%s", shadow)
	}
	if code, err := run("adduser", "-S", "-D", "-H", "svc"); code == 0 || !strings.Contains(err, "user 'svc' in use") {
		t.Fatalf("duplicate user not refused: %d %q", code, err)
	}
}

// TestAddUserWithoutGroupCreatesOne: busybox makes a group of the same name and
// takes the uid as its gid when that id is free.
func TestAddUserWithoutGroupCreatesOne(t *testing.T) {
	dir := fakeEtc(t)
	if code, err := run("adduser", "-D", "-H", "-s", "/bin/sh", "-g", "Test User", "u1"); code != 0 {
		t.Fatalf("adduser: %d %s", code, err)
	}
	if got := read(t, dir, "passwd"); !strings.Contains(got, "u1:x:1000:1000:Test User:/home/u1:/bin/sh") {
		t.Fatalf("passwd line wrong:\n%s", got)
	}
	if got := read(t, dir, "group"); !strings.Contains(got, "u1:x:1000:") {
		t.Fatalf("group line wrong:\n%s", got)
	}
}

// TestMemberForms: adduser USER GROUP and addgroup USER GROUP both append the
// name to the member list.
func TestMemberForms(t *testing.T) {
	dir := fakeEtc(t)
	run("addgroup", "-S", "g1")
	run("adduser", "-S", "-D", "-H", "-G", "g1", "u2")
	if code, err := run("adduser", "u2", "wheel"); code != 0 {
		t.Fatalf("adduser USER GROUP: %d %s", code, err)
	}
	if got := read(t, dir, "group"); !strings.Contains(got, "wheel:x:100:u2") {
		t.Fatalf("member not added:\n%s", got)
	}
	if code, err := run("addgroup", "u2", "nosuch"); code == 0 || !strings.Contains(err, "unknown group nosuch") {
		t.Fatalf("unknown group not reported: %d %q", code, err)
	}
	if code, err := run("addgroup", "ghost", "wheel"); code == 0 || !strings.Contains(err, "unknown user ghost") {
		t.Fatalf("unknown user not reported: %d %q", code, err)
	}
}

// TestDeluser and TestDelgroup cover the removal rules measured on alpine.
func TestDeluser(t *testing.T) {
	dir := fakeEtc(t)
	run("adduser", "-D", "-H", "u3")
	if code, err := run("deluser", "u3"); code != 0 {
		t.Fatalf("deluser: %d %s", code, err)
	}
	if strings.Contains(read(t, dir, "passwd"), "u3:") {
		t.Fatal("passwd still has u3")
	}
	if strings.Contains(read(t, dir, "shadow"), "u3:") {
		t.Fatal("shadow still has u3")
	}
	if strings.Contains(read(t, dir, "group"), "u3:") {
		t.Fatal("the group that belonged to u3 survived")
	}
	if code, err := run("deluser", "u3"); code == 0 || !strings.Contains(err, "unknown user u3") {
		t.Fatalf("second deluser: %d %q", code, err)
	}
}

func TestDelgroup(t *testing.T) {
	dir := fakeEtc(t)
	run("addgroup", "-S", "g2")
	run("adduser", "-S", "-D", "-H", "-G", "g2", "u4")
	if code, err := run("delgroup", "g2"); code == 0 || !strings.Contains(err, "'u4' still has 'g2' as their primary group!") {
		t.Fatalf("primary group not protected: %d %q", code, err)
	}
	if code, err := run("delgroup", "u4", "g2"); code != 0 {
		t.Fatalf("delgroup USER GROUP: %d %s", code, err)
	}
	if got := read(t, dir, "group"); !strings.Contains(got, "g2:x:101:") {
		t.Fatalf("delgroup USER removed the group itself:\n%s", got)
	}
}
