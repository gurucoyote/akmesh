package comm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write puts content into a file of dir and returns its path.
func write(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// runC calls Run and returns stdout, stderr and the status.
func runC(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	var out, errb strings.Builder
	code := Run(args, strings.NewReader(""), &out, &errb)
	return out.String(), errb.String(), code
}

// TestColumns holds GNU 9.7's shapes (measured 23.11.2026, acmesh-y16).
func TestColumns(t *testing.T) {
	dir := t.TempDir()
	a := write(t, dir, "a", "a\nb\n")
	b := write(t, dir, "b", "b\nc\n")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"three columns", []string{a, b}, "a\n\t\tb\n\tc\n"},
		{"suppress 1 and 2", []string{"-12", a, b}, "b\n"},
		{"suppress 1 and 3", []string{"-13", a, b}, "c\n"},
		{"suppress 2 and 3", []string{"-23", a, b}, "a\n"},
		{"suppress all", []string{"-123", a, b}, ""},
		{"total", []string{"--total", a, b}, "a\n\t\tb\n\tc\n1\t1\t1\ttotal\n"},
		{"delimiter", []string{"--output-delimiter=:", a, b}, "a\n::b\n:c\n"},
	}
	for _, c := range cases {
		got, errOut, code := runC(t, c.args...)
		if code != 0 || errOut != "" {
			t.Fatalf("%s: code %d, err %q", c.name, code, errOut)
		}
		if got != c.want {
			t.Fatalf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestEmptyCommonLine: u-root dropped a record whose output was two tabs, which
// is exactly an empty line present in both files. GNU prints it.
func TestEmptyCommonLine(t *testing.T) {
	dir := t.TempDir()
	a := write(t, dir, "a", "a\n\nb\n")
	b := write(t, dir, "b", "a\n\nb\n")
	got, _, code := runC(t, a, b)
	if code != 0 || got != "\t\ta\n\t\t\n\t\tb\n" {
		t.Fatalf("code %d, got %q", code, got)
	}
}

// TestCheckOrder: --check-order fails on an unsorted file, the default accepts.
func TestCheckOrder(t *testing.T) {
	dir := t.TempDir()
	a := write(t, dir, "a", "b\na\n")
	b := write(t, dir, "b", "a\n")
	if _, _, code := runC(t, a, b); code != 0 {
		t.Fatalf("default: exit %d", code)
	}
	_, errOut, code := runC(t, "--check-order", a, b)
	if code != 1 || !strings.Contains(errOut, "not in sorted order") {
		t.Fatalf("--check-order: code %d, err %q", code, errOut)
	}
}
