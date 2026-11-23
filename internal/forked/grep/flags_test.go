package grep

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMaxCount: -m stops after N matching lines per file, as GNU does.
func TestMaxCount(t *testing.T) {
	out, _, code := runGrep(t, []string{"-m", "2", "x"}, "x1\nx2\nx3\nx4\n")
	if code != 0 || out != "x1\nx2\n" {
		t.Fatalf("grep -m 2 = %d, %q", code, out)
	}
	// glued form, the way agents write it
	out, _, _ = runGrep(t, []string{"-m1", "x"}, "x1\nx2\n")
	if out != "x1\n" {
		t.Fatalf("grep -m1 = %q", out)
	}
	// the count is per file, not per invocation
	dir := t.TempDir()
	for _, n := range []string{"a.txt", "b.txt"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("x\nx\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out, _, _ = runGrep(t, []string{"-m", "1", "x", filepath.Join(dir, "a.txt"), filepath.Join(dir, "b.txt")}, "")
	if strings.Count(out, ":x\n") != 2 || strings.Count(strings.ReplaceAll(out, ":x\n", ""), "x\n") != 0 {
		t.Fatalf("grep -m 1 over two files = %q, want exactly one line per file", out)
	}
}

// TestWholeLine: -x anchors the pattern, so a substring does not match.
func TestWholeLine(t *testing.T) {
	out, _, code := runGrep(t, []string{"-x", "cat"}, "cat\nconcat\ncat and dog\n")
	if code != 0 || out != "cat\n" {
		t.Fatalf("grep -x cat = %d, %q", code, out)
	}
	// with a BRE quantifier inside
	out, _, _ = runGrep(t, []string{"-x", `a\+b`}, "aab\nxaab\n")
	if out != "aab\n" {
		t.Fatalf("grep -x 'a\\+b' = %q", out)
	}
}

// TestWithFilename: -H prints the name even for a single file.
func TestWithFilename(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "one.txt")
	if err := os.WriteFile(name, []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, _ := runGrep(t, []string{"-H", "hit", name}, "")
	if out != name+":hit\n" {
		t.Fatalf("grep -H = %q", out)
	}
}

// TestNullData: -z reads and writes NUL-separated records.
func TestNullData(t *testing.T) {
	out, _, code := runGrep(t, []string{"-z", "b"}, "a\x00b\x00c\x00")
	if code != 0 || out != "b\x00" {
		t.Fatalf("grep -z = %d, %q", code, out)
	}
	// a record containing a newline is one record under -z
	out, _, _ = runGrep(t, []string{"-z", "^b"}, "a\x00b\nc\x00")
	if out != "b\nc\x00" {
		t.Fatalf("grep -z with a newline inside a record = %q", out)
	}
}

// TestIncludeExclude: the globs filter what a recursive search looks at, and a
// file named on the command line is always searched.
func TestIncludeExclude(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("a.log", "needle\n")
	write("b.txt", "needle\n")
	write("c.log", "needle\n")

	out, _, _ := runGrep(t, []string{"-r", "--include=*.log", "needle", dir}, "")
	if strings.Contains(out, "b.txt") || !strings.Contains(out, "a.log") || !strings.Contains(out, "c.log") {
		t.Fatalf("--include=*.log = %q", out)
	}
	out, _, _ = runGrep(t, []string{"-r", "--exclude=*.log", "needle", dir}, "")
	if strings.Contains(out, ".log") || !strings.Contains(out, "b.txt") {
		t.Fatalf("--exclude=*.log = %q", out)
	}
	// the file named directly is searched even when it does not match the glob
	out, _, code := runGrep(t, []string{"--include=*.log", "needle", filepath.Join(dir, "b.txt")}, "")
	if code != 0 || out != "needle\n" {
		t.Fatalf("--include on an explicit file = %d, %q", code, out)
	}
}

// TestTextMode: -a is accepted; there is no binary detection, so the answer is
// the same as GNU's -a.
func TestTextMode(t *testing.T) {
	out, _, code := runGrep(t, []string{"-a", "needle"}, "\x00needle\x00\n")
	if code != 0 || !strings.Contains(out, "needle") {
		t.Fatalf("grep -a = %d, %q", code, out)
	}
}
