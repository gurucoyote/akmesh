//go:build !windows

package login

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeSystem points the account database and the two system calls at capture
// points. A test must never really change its own uid.
func fakeSystem(t *testing.T, passwd string) (ids *[]int, exec *[][]string, env *[]string) {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, "passwd")
	if err := os.WriteFile(file, []byte(passwd), 0o644); err != nil {
		t.Fatal(err)
	}
	oldPasswd, oldIDs, oldExec := passwdFile, setIDs, execShell
	gotIDs := []int{}
	idArgs := &gotIDs
	gotExec := [][]string{}
	gotEnv := []string{}
	passwdFile = file
	setIDs = func(u, g int) error { *idArgs = []int{u, g}; return nil }
	execShell = func(path string, argv, e []string) error {
		_ = path
		gotExec = append(gotExec, argv)
		gotEnv = e
		return nil
	}
	oldDir, _ := os.Getwd()
	t.Cleanup(func() {
		passwdFile, setIDs, execShell = oldPasswd, oldIDs, oldExec
		os.Chdir(oldDir)
	})
	return idArgs, &gotExec, &gotEnv
}

// TestLoginSwitchesToTheAccount is the point of the command: the ids come from
// /etc/passwd, the home directory becomes the cwd and the shell is called with
// -l, which is what makes it read the profiles.
func TestLoginSwitchesToTheAccount(t *testing.T) {
	home := t.TempDir()
	ids, exec, env := fakeSystem(t, "root:x:0:0:root:/root:/bin/sh\n"+
		"martin:x:1000:1000:Martin:"+home+":/bin/sh\n")

	var out, errb bytes.Buffer
	if code := Run([]string{"-f", "martin"}, strings.NewReader(""), &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if (*ids)[0] != 1000 || (*ids)[1] != 1000 {
		t.Fatalf("ids %v, want uid 1000 gid 1000", *ids)
	}
	if len(*exec) != 1 {
		t.Fatalf("shell not started: %v", *exec)
	}
	call := (*exec)[0]
	if len(call) < 2 || call[0] != "/bin/sh" || call[1] != "-l" {
		t.Fatalf("shell called as %q, want /bin/sh -l", call)
	}
	joined := strings.Join(*env, "\n")
	for _, want := range []string{"HOME=" + home, "USER=martin", "LOGNAME=martin", "SHELL=/bin/sh"} {
		if !strings.Contains(joined, want) {
			t.Errorf("environment misses %q, got:\n%s", want, joined)
		}
	}
	if wd, _ := os.Getwd(); wd != home {
		t.Errorf("cwd %q, want the home directory %q", wd, home)
	}
}

// TestLoginKeepsTheCallersPath: the caller's PATH survives, because a login on a
// console inherits what the getty had.
func TestLoginKeepsTheCallersPath(t *testing.T) {
	home := t.TempDir()
	_, _, env := fakeSystem(t, "martin:x:1000:1000::"+home+":/bin/sh\n")
	t.Setenv("PATH", "/keep/me")

	var out, errb bytes.Buffer
	if code := Run([]string{"-f", "martin"}, strings.NewReader(""), &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	if !strings.Contains(strings.Join(*env, "\n"), "PATH=/keep/me") {
		t.Fatalf("PATH not kept: %v", *env)
	}
}

// TestLoginOverridesCallerHome: the session must not inherit another
// user's HOME.
func TestLoginOverridesCallerHome(t *testing.T) {
	home := t.TempDir()
	_, _, env := fakeSystem(t, "martin:x:1000:1000::"+home+":/bin/sh\n")
	t.Setenv("HOME", "/elsewhere")

	var out, errb bytes.Buffer
	if code := Run([]string{"-f", "martin"}, strings.NewReader(""), &out, &errb); code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
	n := 0
	for _, kv := range *env {
		if strings.HasPrefix(kv, "HOME=") {
			n++
			if kv != "HOME="+home {
				t.Fatalf("HOME %q, want %q", kv, "HOME="+home)
			}
		}
	}
	if n != 1 {
		t.Fatalf("%d HOME entries, want exactly one: %v", n, *env)
	}
}

// TestLoginRefusesWithoutForce: there is no password check in this build, and a
// login that pretends to authenticate is worse than one that says so.
func TestLoginRefusesWithoutForce(t *testing.T) {
	home := t.TempDir()
	fakeSystem(t, "martin:x:1000:1000::"+home+":/bin/sh\n")

	var out, errb bytes.Buffer
	if code := Run([]string{"martin"}, strings.NewReader(""), &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "password") {
		t.Fatalf("message %q does not name the reason", errb.String())
	}
}

// TestLoginUnknownUser: a name that is not in the database is an error, never a
// silent numeric fallback (that is right for id, wrong for login).
func TestLoginUnknownUser(t *testing.T) {
	fakeSystem(t, "root:x:0:0::/root:/bin/sh\n")

	var out, errb bytes.Buffer
	if code := Run([]string{"-f", "gibtsnicht"}, strings.NewReader(""), &out, &errb); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errb.String(), "unknown user") {
		t.Fatalf("message %q", errb.String())
	}
}

// TestLoginHelp: the help audit asks every command, and help must end with 0.
func TestLoginHelp(t *testing.T) {
	var out, errb bytes.Buffer
	if code := Run([]string{"--help"}, strings.NewReader(""), &out, &errb); code != 0 {
		t.Fatalf("exit %d, want 0", code)
	}
	if !strings.Contains(out.String(), "usage: login") {
		t.Fatalf("output %q", out.String())
	}
}
