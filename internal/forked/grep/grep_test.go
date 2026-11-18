package grep

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runGrep calls Run the way the dispatcher does.
func runGrep(t *testing.T, args []string, input string) (string, string, int) {
	t.Helper()
	var out, errOut strings.Builder
	code := Run(args, strings.NewReader(input), &out, &errOut)
	return out.String(), errOut.String(), code
}

// TestExtendedRegexpAndWordMatch: -E needs no backslashes for +, -w keeps the
// match on word boundaries.
func TestExtendedRegexpAndWordMatch(t *testing.T) {
	out, _, code := runGrep(t, []string{"-E", "a+b"}, "aaab\nxx\n")
	if code != 0 || out != "aaab\n" {
		t.Fatalf("grep -E = %d, %q", code, out)
	}
	out, _, _ = runGrep(t, []string{"-w", "cat"}, "cat\nconcat\n")
	if out != "cat\n" {
		t.Fatalf("grep -w = %q, want only the whole word", out)
	}
}

// TestContextLines: -A and -B print neighbours, groups separated by "--", and a
// line that is both after-context and before-context appears once.
func TestContextLines(t *testing.T) {
	input := "one\ntwo\nMATCH\nfour\nfive\nMATCH\nseven\n"
	out, _, _ := runGrep(t, []string{"-A", "1", "MATCH"}, input)
	want := "MATCH\nfour\n--\nMATCH\nseven\n"
	if out != want {
		t.Fatalf("grep -A 1 = %q, want %q", out, want)
	}
	// With -C 1 the two windows overlap, so GNU prints them as one group and
	// no "--" appears. Checked against GNU grep 3.11 in the container.
	out, _, _ = runGrep(t, []string{"-C", "1", "MATCH"}, input)
	want = "two\nMATCH\nfour\nfive\nMATCH\nseven\n"
	if out != want {
		t.Fatalf("grep -C 1 = %q, want %q", out, want)
	}
}

// TestOnlyMatchAndNumbers: -o prints each match, -n adds the line number.
func TestOnlyMatchAndNumbers(t *testing.T) {
	out, _, _ := runGrep(t, []string{"-o", "-n", "ab"}, "ab ab\nxx\n")
	if out != "1:ab\n1:ab\n" {
		t.Fatalf("grep -o -n = %q", out)
	}
}

// TestExitStatus: 0 when something matched, 1 when nothing did, 2 for a broken
// pattern. Upstream returned 0 for "no match", which a shell script misreads.
func TestExitStatus(t *testing.T) {
	if _, _, code := runGrep(t, []string{"nope"}, "a\nb\n"); code != 1 {
		t.Fatalf("no match exit %d, want 1", code)
	}
	if _, errOut, code := runGrep(t, []string{"("}, "a\n"); code != 2 || !strings.Contains(errOut, "grep:") {
		t.Fatalf("bad pattern exit %d, stderr %q; want 2 with a message", code, errOut)
	}
}

// TestRecursiveNamesTheFile: -r prints the path prefix and keeps the "./" that
// GNU prints for the start path ".".
func TestRecursiveNamesTheFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("hit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	out, _, code := runGrep(t, []string{"-r", "hit", "."}, "")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "./a.txt:hit") || !strings.Contains(out, "./sub/b.txt:hit") {
		t.Fatalf("grep -r = %q, want both files with the ./ prefix", out)
	}
}

// TestCountPerFile: -c prints one count per file, with the name when there is
// more than one file.
func TestCountPerFile(t *testing.T) {
	dir := t.TempDir()
	one := filepath.Join(dir, "one.txt")
	two := filepath.Join(dir, "two.txt")
	if err := os.WriteFile(one, []byte("a\nb\na\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(two, []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, _ := runGrep(t, []string{"-c", "a", one, two}, "")
	if !strings.Contains(out, one+":2") || !strings.Contains(out, two+":0") {
		t.Fatalf("grep -c = %q, want one count per file", out)
	}
}
