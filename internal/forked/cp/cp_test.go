package cp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runCp(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code := Run(args, strings.NewReader(""), &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestArchiveKeepsModeAndTime: -a is the reason cp got forked. Mode and
// modification time have to survive, otherwise a copied script loses +x.
func TestArchiveKeepsModeAndTime(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(src, "a.sh")
	if err := os.WriteFile(file, []byte("#!/bin/sh\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(file, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "dst")
	if _, errOut, code := runCp(t, "-a", src, dst); code != 0 {
		t.Fatalf("cp -a exit %d, stderr %q", code, errOut)
	}
	info, err := os.Stat(filepath.Join(dst, "a.sh"))
	if err != nil {
		t.Fatalf("nested file missing: %v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode %04o, want 0640", info.Mode().Perm())
	}
	if !info.ModTime().Equal(stamp) {
		t.Fatalf("mtime %v, want %v", info.ModTime(), stamp)
	}
}

// TestTargetDirectory: -t DIR is GNU's spelling for "these all go in there".
func TestTargetDirectory(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	target := filepath.Join(dir, "out")
	if err := os.WriteFile(a, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, errOut, code := runCp(t, "-t", target, a, b); code != 0 {
		t.Fatalf("cp -t exit %d, stderr %q", code, errOut)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(target, name)); err != nil {
			t.Fatalf("%s missing in the target dir", name)
		}
	}
}

// TestNoClobber: -n leaves the existing file alone; without it the copy wins.
func TestNoClobber(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dst := filepath.Join(dir, "dst.txt")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code := runCp(t, "-n", src, dst); code != 0 {
		t.Fatalf("cp -n exit %d", code)
	}
	got, _ := os.ReadFile(dst)
	if string(got) != "old" {
		t.Fatalf("-n overwrote the file: %q", got)
	}
	if _, _, code := runCp(t, src, dst); code != 0 {
		t.Fatalf("cp exit %d", code)
	}
	got, _ = os.ReadFile(dst)
	if string(got) != "new" {
		t.Fatalf("plain cp did not overwrite: %q", got)
	}
}

// TestMissingSourceSaysItTheGnuWay: the message an agent reads has to name the
// source like GNU does, and the exit status is 1.
func TestMissingSourceSaysItTheGnuWay(t *testing.T) {
	dir := t.TempDir()
	_, errOut, code := runCp(t, filepath.Join(dir, "nix"), filepath.Join(dir, "x"))
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut, "cannot stat") {
		t.Fatalf("stderr %q, want GNU's cannot stat wording", errOut)
	}
}
