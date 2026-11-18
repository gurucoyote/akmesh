//go:build linux && amd64

package tracedeps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFindsTheFileAScriptReads is the case the command exists for: a script
// calls cat, and both the program and the file it opens have to appear in the
// list, or an image built from it does not run.
func TestFindsTheFileAScriptReads(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "inhalt.txt")
	if err := os.WriteFile(data, []byte("hallo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(dir, "k.sh")
	body := "#!/bin/sh\ncat " + data + "\n"
	if err := os.WriteFile(script, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}

	out, errFile, code := runHere(t, []string{script})
	if code != 0 {
		t.Fatalf("exit: want 0, got %d (%s)", code, errFile)
	}
	if !strings.Contains(out, data) {
		t.Errorf("the file the script read is missing from the list:\n%s", out)
	}
	if !strings.Contains(out, script) {
		t.Errorf("the script itself is missing from the list:\n%s", out)
	}
}

// TestProbesAreHiddenByDefault: a PATH search asks about four paths before it
// finds one, and those are not files the image needs.
func TestProbesAreHiddenByDefault(t *testing.T) {
	dir := t.TempDir()
	out, _, code := runHere(t, []string{"cat", "/dev/null"})
	if code != 0 {
		t.Fatalf("exit: want 0, got %d", code)
	}
	if !strings.Contains(out, "/bin/cat") && !strings.Contains(out, "/usr/bin/cat") {
		t.Errorf("the binary is not in the list:\n%s", out)
	}
	if strings.Contains(out, "/usr/local/sbin/cat") {
		t.Errorf("a path that was only searched is in the list:\n%s", out)
	}
	_ = dir
}

// TestUsageWithoutProgram: no program is a usage line, not a trace of nothing.
func TestUsageWithoutProgram(t *testing.T) {
	_, errFile, code := runHere(t, nil)
	if code != 2 {
		t.Fatalf("exit: want 2, got %d", code)
	}
	if !strings.Contains(errFile, "usage:") {
		t.Errorf("no usage line: %q", errFile)
	}
}

// runHere runs tracedeps with args and returns its stdout, stderr and code.
func runHere(t *testing.T, args []string) (string, string, int) {
	t.Helper()
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "out"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	errFile, err := os.Create(filepath.Join(dir, "err"))
	if err != nil {
		t.Fatal(err)
	}
	defer errFile.Close()
	code := Run(args, nil, out, errFile)
	o, _ := os.ReadFile(out.Name())
	e, _ := os.ReadFile(errFile.Name())
	return string(o), string(e), code
}
