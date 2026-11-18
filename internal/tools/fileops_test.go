package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These four commands had no test at all (ticket acmesh-nyu). Each test below
// states the claim an agent relies on, not just "it ran".

// TestStatFormatDirectives pins the format an agent parses: %a is the octal
// mode without a leading zero, %s the size, %n the name.
func TestStatFormatDirectives(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := captureIn(t, runStat, []string{"-c", "%a %n", path}, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if want := "644 " + path + "\n"; out != want {
		t.Fatalf("got %q, want %q", out, want)
	}

	out, code = captureIn(t, runStat, []string{"-c", "%s", path}, "")
	if code != 0 || out != "6\n" {
		t.Fatalf("size = %d, %q; want 0, \"6\\n\"", code, out)
	}

	out, code = captureIn(t, runStat, []string{"-c", "%F", path}, "")
	if code != 0 || out != "regular file\n" {
		t.Fatalf("type = %d, %q", code, out)
	}

	// The default form is the block GNU prints: an agent greps for "File:",
	// "Size:" and the mode, so those labels have to be there.
	out, code = captureIn(t, runStat, []string{path}, "")
	if code != 0 || !strings.Contains(out, "File: "+path) || !strings.Contains(out, "Size: 6") || !strings.Contains(out, "(0644/-rw-r--r--)") {
		t.Fatalf("default output %q lacks the GNU labels File, Size and mode", out)
	}
}

func TestStatErrors(t *testing.T) {
	if _, code := captureIn(t, runStat, nil, ""); code != 2 {
		t.Fatalf("no operand: exit %d, want 2", code)
	}
	if _, code := captureIn(t, runStat, []string{"-Z"}, ""); code != 2 {
		t.Fatalf("unknown option: exit %d, want 2", code)
	}
	if _, code := captureIn(t, runStat, []string{"/nicht/vorhanden"}, ""); code != 1 {
		t.Fatalf("missing file: exit %d, want 1 (a missing file is not a usage error)", code)
	}
}

// TestSyncFlushesTheNamedFile is the claim that matters: the call really
// reaches the file. The failure path proves the error is reported, so a sync
// that silently swallowed its error would not pass.
func TestSyncFlushesTheNamedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, code := captureIn(t, runSync, []string{path}, ""); code != 0 {
		t.Fatalf("sync on a writable file: exit %d, want 0", code)
	}
	if _, code := captureIn(t, runSync, []string{filepath.Join(dir, "fehlt")}, ""); code != 1 {
		t.Fatalf("sync on a missing file: exit %d, want 1", code)
	}
	// Without operands the answer is platform-dependent: Linux syncs, Windows
	// has no whole-system call and says so with exit 1.
	_, code := captureIn(t, runSync, nil, "")
	switch runtime.GOOS {
	case "windows":
		if code != 1 {
			t.Fatalf("windows sync: exit %d, want 1 (no whole-system sync there)", code)
		}
	default:
		if code != 0 {
			t.Fatalf("%s sync: exit %d, want 0", runtime.GOOS, code)
		}
	}
}

// TestHostnamePrintsTheKernelName pins both directions: the name is printed,
// and setting one is refused instead of silently ignored.
func TestHostnamePrintsTheKernelName(t *testing.T) {
	want, err := os.Hostname()
	if err != nil {
		t.Skip("no hostname on this platform")
	}
	out, code := captureIn(t, runHostname, nil, "")
	if code != 0 || strings.TrimSpace(out) != want {
		t.Fatalf("hostname = %d, %q; want 0, %q", code, strings.TrimSpace(out), want)
	}
	if _, code := captureIn(t, runHostname, []string{"neu"}, ""); code != 2 {
		t.Fatalf("hostname with an operand: exit %d, want 2", code)
	}
}

// TestPolicyCommandExplainsItself covers the command a harness calls to see
// what is enforced. Without ACMESH_POLICY the process is unrestricted, and the
// exit code says so (1), which is the signal a harness reads.
func TestPolicyCommandExplainsItself(t *testing.T) {
	dir := t.TempDir()
	readable := filepath.Join(dir, "inside.txt")
	if err := os.WriteFile(readable, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code := captureIn(t, runPolicy, nil, "")
	if code != 1 || !strings.Contains(out, "no policy") {
		t.Fatalf("no policy = %d, %q; want exit 1 and the words \"no policy\"", code, out)
	}

	out, code = captureIn(t, runPolicy, []string{"--help"}, "")
	if code != 0 || !strings.Contains(out, "--check") {
		t.Fatalf("--help = %d, %q", code, out)
	}

	out, code = captureIn(t, runPolicy, []string{"--check", readable}, "")
	if code != 0 || !strings.HasPrefix(out, "allowed: ") {
		t.Fatalf("--check on a readable path = %d, %q; want 0 and \"allowed:\"", code, out)
	}

	out, code = captureIn(t, runPolicy, []string{"--check", filepath.Join(dir, "weg")}, "")
	if code != 1 || !strings.HasPrefix(out, "denied: ") {
		t.Fatalf("--check on a missing path = %d, %q; want 1 and \"denied:\"", code, out)
	}

	if _, code := captureIn(t, runPolicy, []string{"--quatsch"}, ""); code != 2 {
		t.Fatalf("unknown argument: exit %d, want 2", code)
	}
}
