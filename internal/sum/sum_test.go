package sum

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runSum(t *testing.T, kind, stdin string, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code := Run(kind, args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestCheckOKAndFailed: -c verifies a list and says OK or FAILED per file, the
// two words every script greps for.
func TestCheckOKAndFailed(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.txt")
	if err := os.WriteFile(good, []byte("hallo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "sums.txt")
	// The sha256 of "hallo\n" and one wrong digest for the same file.
	content := "622cb3371c1a08096eaac564fb59acccda1fcdbe13a9dd10b486e6463c8c2525  " + good + "\n" +
		"0000000000000000000000000000000000000000000000000000000000000000  " + good + "\n"
	if err := os.WriteFile(list, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out, errOut, code := runSum(t, "sha256", "", "-c", list)
	if code != 1 {
		t.Fatalf("one bad line must exit 1, got %d", code)
	}
	if !strings.Contains(out, good+": OK") {
		t.Fatalf("output %q misses the OK line", out)
	}
	if !strings.Contains(out, good+": FAILED") {
		t.Fatalf("output %q misses the FAILED line", out)
	}
	if !strings.Contains(errOut, "did NOT match") {
		t.Fatalf("stderr %q misses the warning", errOut)
	}
}

// TestCheckAllGood: a list where every line matches exits 0.
func TestCheckAllGood(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(file, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "l.txt")
	if err := os.WriteFile(list, []byte("a9993e364706816aba3e25717850c26c9cd0d89d  "+file+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := runSum(t, "sha1", "", "-c", list)
	if code != 0 || !strings.Contains(out, file+": OK") {
		t.Fatalf("-c = %d, %q", code, out)
	}
}

// TestCheckStarSeparator: GNU also writes "hex *name" for binary mode; the star
// must not become part of the file name.
func TestCheckStarSeparator(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(file, []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "l.txt")
	if err := os.WriteFile(list, []byte("a9993e364706816aba3e25717850c26c9cd0d89d *"+file+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, errOut, code := runSum(t, "sha1", "", "-c", list)
	if code != 0 {
		t.Fatalf("-c with * separator exit %d, stderr %q", code, errOut)
	}
}

// TestCheckMissingFile: a file named in the list but not on disk is a failure,
// not a silent pass.
func TestCheckMissingFile(t *testing.T) {
	dir := t.TempDir()
	list := filepath.Join(dir, "l.txt")
	missing := filepath.Join(dir, "nix.txt")
	if err := os.WriteFile(list, []byte("0000000000000000000000000000000000000000000000000000000000000000  "+missing+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, code := runSum(t, "sha256", "", "-c", list)
	if code != 1 || !strings.Contains(out, "FAILED open or read") {
		t.Fatalf("-c on a missing file = %d, %q", code, out)
	}
}

// TestBareRun: without -c the output keeps the "hex  name" shape.
func TestBareRun(t *testing.T) {
	out, _, code := runSum(t, "sha256", "hallo\n")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.HasPrefix(out, "622cb3371c1a08096eaac564fb59acccda1fcdbe13a9dd10b486e6463c8c2525  -") {
		t.Fatalf("output %q", out)
	}
}
