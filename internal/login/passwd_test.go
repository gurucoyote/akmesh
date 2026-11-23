//go:build !windows

package login

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAskTwiceReadsBothLines: both answers have to survive on one pipe, which is
// how an agent feeds passwd. Two readers swallowed the second line once and
// produced "passwd: EOF".
func TestAskTwiceReadsBothLines(t *testing.T) {
	var out bytes.Buffer
	first, second, err := askTwice(strings.NewReader("eins\nzwei\n"), &out)
	if err != nil {
		t.Fatalf("askTwice: %v", err)
	}
	if first != "eins" || second != "zwei" {
		t.Fatalf("got %q/%q, want eins/zwei", first, second)
	}
}

// TestRunMkpasswdMatchesOpenSSL pins the hashes against openssl's output for the
// same salt and password, so a wrong SHA-crypt cannot creep back in.
func TestRunMkpasswdMatchesOpenSSL(t *testing.T) {
	cases := []struct{ method, want string }{
		{"sha-512", "$6$salz$iJH8hiUTnWErNH4Gims/SDm/O1x/LxwMym0qqmscGC6GEWrrvcCr599Mf7udFdAJKhDo0mOHZYGwFk..Zc2TF."},
		{"sha-256", "$5$salz$9ftDxp3UfTxR6nG9kRLaOFPeWbqVuOwNUhrw7j4Zaw."},
	}
	for _, c := range cases {
		var out, errb bytes.Buffer
		code := RunMkpasswd("mkpasswd", []string{"-m", c.method, "-s", "salz", "geheim"}, nil, &out, &errb)
		if code != 0 {
			t.Fatalf("%s: exit %d, %s", c.method, code, errb.String())
		}
		got := strings.TrimSpace(out.String())
		if got != c.want {
			t.Fatalf("%s: got %q, want %q", c.method, got, c.want)
		}
	}
}

// TestRunMkpasswdRefusesOtherMethods: an unsupported method must say so instead
// of writing a hash that no login will ever accept.
func TestRunMkpasswdRefusesOtherMethods(t *testing.T) {
	var out, errb bytes.Buffer
	if code := RunMkpasswd("mkpasswd", []string{"-m", "yescrypt", "x"}, nil, &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "not supported") {
		t.Fatalf("message: %q", errb.String())
	}
}

// TestPasswdRewritesOnlyTheHash: the line keeps its other fields and the file
// keeps its other lines.
func TestPasswdRewritesOnlyTheHash(t *testing.T) {
	dir := t.TempDir()
	shadow := filepath.Join(dir, "shadow")
	content := "root:!:19000:0:99999:7:::\nanderer:$6$alt$hash:19000:0:99999:7:::\n"
	if err := os.WriteFile(shadow, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	old := shadowFile
	shadowFile = shadow
	defer func() { shadowFile = old }()

	var out, errb bytes.Buffer
	if code := RunPasswd("passwd", []string{"root"}, strings.NewReader("geheim\ngeheim\n"), &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	data, err := os.ReadFile(shadow)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines: %q", lines)
	}
	if !strings.HasPrefix(lines[0], "root:$6$") {
		t.Fatalf("root line was not rewritten: %q", lines[0])
	}
	if !strings.HasSuffix(lines[0], ":19000:0:99999:7:::") {
		t.Fatalf("the fields after the hash changed: %q", lines[0])
	}
	if lines[1] != "anderer:$6$alt$hash:19000:0:99999:7:::" {
		t.Fatalf("the other account was touched: %q", lines[1])
	}
	// The rewritten file is root readable only.
	if fi, err := os.Stat(shadow); err == nil && fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v, want 0600", fi.Mode().Perm())
	}
}

// TestPasswdRejectsMismatch: two different passwords change nothing.
func TestPasswdRejectsMismatch(t *testing.T) {
	dir := t.TempDir()
	shadow := filepath.Join(dir, "shadow")
	if err := os.WriteFile(shadow, []byte("root:!:19000:0:99999:7:::\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := shadowFile
	shadowFile = shadow
	defer func() { shadowFile = old }()

	var out, errb bytes.Buffer
	if code := RunPasswd("passwd", []string{"root"}, strings.NewReader("eins\nzwei\n"), &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	data, _ := os.ReadFile(shadow)
	if !strings.Contains(string(data), "root:!") {
		t.Fatalf("the file was changed although the passwords differed: %q", data)
	}
}
