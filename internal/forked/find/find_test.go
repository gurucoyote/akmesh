package find

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// runFind calls Run the way the dispatcher does and returns the lines.
func runFind(t *testing.T, args []string, dir string) ([]string, string, int) {
	t.Helper()
	var out, errOut strings.Builder
	wd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(wd)
	code := Run(args, strings.NewReader(""), &out, &errOut)
	lines := strings.Fields(out.String())
	sort.Strings(lines)
	return lines, errOut.String(), code
}

// tree builds a small hierarchy: a.txt, sub/b.txt, sub/deep/c.log.
func tree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"a.txt": "x", "sub/b.txt": "y", "sub/deep/c.log": "z"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// TestStartPathIsOptional and the "./" prefix are the two things agents type
// without thinking: `find -name x` and `find . -name x`.
func TestStartPathIsOptional(t *testing.T) {
	dir := tree(t)
	lines, errOut, code := runFind(t, []string{"-name", "a.txt"}, dir)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if len(lines) != 1 || lines[0] != "./a.txt" {
		t.Fatalf("got %v, want ./a.txt", lines)
	}
	lines, _, _ = runFind(t, []string{".", "-name", "b.txt"}, dir)
	if len(lines) != 1 || lines[0] != "./sub/b.txt" {
		t.Fatalf("got %v, want ./sub/b.txt", lines)
	}
}

// TestMaxDepthReportsTheBoundaryDirectory: GNU prints a directory that sits on
// maxdepth and stops there. Skipping it silently was the first bug of this fork.
func TestMaxDepthReportsTheBoundaryDirectory(t *testing.T) {
	dir := tree(t)
	lines, _, code := runFind(t, []string{".", "-maxdepth", "1", "-type", "d"}, dir)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if len(lines) != 2 || lines[0] != "." || lines[1] != "./sub" {
		t.Fatalf("got %v, want . and ./sub", lines)
	}
	lines, _, _ = runFind(t, []string{".", "-maxdepth", "1", "-name", "*.txt"}, dir)
	if len(lines) != 1 || lines[0] != "./a.txt" {
		t.Fatalf("got %v, want only ./a.txt", lines)
	}
}

// TestMindepth: -mindepth 2 leaves out the top level.
func TestMindepth(t *testing.T) {
	dir := tree(t)
	lines, _, _ := runFind(t, []string{".", "-mindepth", "2", "-type", "f"}, dir)
	if len(lines) != 2 || lines[0] != "./sub/b.txt" || lines[1] != "./sub/deep/c.log" {
		t.Fatalf("got %v, want the two deeper files", lines)
	}
}

// TestNotAndType: -not negates the following predicate only.
func TestNotAndType(t *testing.T) {
	dir := tree(t)
	lines, _, _ := runFind(t, []string{".", "-type", "f", "-not", "-name", "*.log"}, dir)
	if len(lines) != 2 {
		t.Fatalf("got %v, want a.txt and sub/b.txt", lines)
	}
	for _, l := range lines {
		if strings.HasSuffix(l, ".log") {
			t.Fatalf("%q should have been excluded", l)
		}
	}
}

// TestExecRunsOncePerMatch: -exec with {} replaced, and no implicit print.
func TestExecRunsOncePerMatch(t *testing.T) {
	dir := tree(t)
	lines, errOut, code := runFind(t, []string{".", "-name", "*.txt", "-exec", "echo", "HIT", "{}", ";"}, dir)
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if len(lines) != 4 { // two hits, each "HIT" plus the path
		t.Fatalf("got %v, want two HIT lines with their paths", lines)
	}
}

// TestErrorsAreNotSuccess: a missing start path and an unknown predicate fail.
func TestErrorsAreNotSuccess(t *testing.T) {
	dir := tree(t)
	if _, _, code := runFind(t, []string{"/nicht/vorhanden"}, dir); code != 1 {
		t.Fatalf("missing path exit %d, want 1", code)
	}
	if _, errOut, code := runFind(t, []string{"-Z"}, dir); code != 2 || !strings.Contains(errOut, "usage") {
		t.Fatalf("unknown predicate exit %d, stderr %q; want 2 with usage", code, errOut)
	}
}
