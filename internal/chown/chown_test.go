//go:build !windows

package chown

import (
	"bytes"
	"os"
	"syscall"
	"testing"
)

func owner(t *testing.T, path string) (int, int) {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("no syscall stat for %s", path)
	}
	return int(st.Uid), int(st.Gid)
}

// TestChownNumeric: the numeric form works without /etc/passwd, which is what an
// unpacked archive needs.
func TestChownNumeric(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing the owner needs root")
	}
	dir := t.TempDir()
	file := dir + "/f.txt"
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := Run("chown", []string{"12345:12345", file}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	uid, gid := owner(t, file)
	if uid != 12345 || gid != 12345 {
		t.Fatalf("owner %d:%d, want 12345:12345", uid, gid)
	}
}

// TestChownRecursive: -R reaches files inside directories, and the group-only
// form leaves the owner alone.
func TestChownRecursive(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("changing the owner needs root")
	}
	dir := t.TempDir()
	sub := dir + "/sub"
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sub+"/inner", []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	if code := Run("chown", []string{"-R", ":777", dir}, nil, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	if _, gid := owner(t, sub+"/inner"); gid != 777 {
		t.Fatalf("inner gid %d, want 777", gid)
	}
	if uid, _ := owner(t, sub); uid != 0 {
		t.Fatalf("owner changed by a group-only call: uid %d", uid)
	}
}

// TestChownUnknownName: a name that is not in the tables is an error with the
// name in it, not a silent no-op.
func TestChownUnknownName(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run("chown", []string{"gibtsnicht-benutzer", t.TempDir()}, nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !bytes.Contains(errb.Bytes(), []byte("gibtsnicht-benutzer")) {
		t.Fatalf("the message does not name the user: %q", errb.String())
	}
}

// TestChownUsage: no arguments is a usage error, as in coreutils.
func TestChownUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run("chown", nil, nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
}
